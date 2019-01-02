package users

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/model/acl"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/util"
	"github.com/gorilla/mux"
)

func (u *users) getUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		username, _, _ := req.BasicAuth()

		// check the request context
		if reqUser, err := user.FromContext(ctx); err == nil {
			rawUser, err := json.Marshal(*reqUser)
			if err != nil {
				msg := "error parsing the context user object"
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, rawUser, http.StatusOK)
			return
		}

		// fetch the user from elasticsearch
		rawUser, err := u.es.getRawUser(req.Context(), username)
		if err != nil {
			msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
		return
	}
}

func (u *users) getUserWithUsername() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username, ok := vars["username"]
		if !ok {
			util.WriteBackError(w, `can't get a user without a "username"`, http.StatusBadRequest)
			return
		}

		rawUser, err := u.es.getRawUser(req.Context(), username)
		if err != nil {
			msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
	}
}

func (u *users) postUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		opts := []user.Options{
			user.SetEmail(userBody.Email),
		}
		if userBody.IsAdmin != nil {
			opts = append(opts, user.SetIsAdmin(*userBody.IsAdmin))
		}
		if userBody.Categories != nil {
			opts = append(opts, user.SetCategories(userBody.Categories))
		}
		if userBody.ACLs != nil {
			opts = append(opts, user.SetACLs(userBody.ACLs))
		}
		if userBody.Ops != nil {
			opts = append(opts, user.SetOps(userBody.Ops))
		}
		if userBody.Indices != nil {
			opts = append(opts, user.SetIndices(userBody.Indices))
		}
		if userBody.Username == "" {
			util.WriteBackError(w, `can't create a user without a "username"`, http.StatusBadRequest)
			return
		}
		if userBody.Password == "" {
			util.WriteBackError(w, `user "password" shouldn't be empty`, http.StatusBadRequest)
			return
		}
		newUser, err := user.New(userBody.Username, userBody.Password, opts...)
		if err != nil {
			msg := fmt.Sprintf("an error occurred while creating user: %v", err)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		rawUser, err := json.Marshal(*newUser)
		if err != nil {
			msg := fmt.Sprintf(`an error occurred while creating a user with "username"="%s"`, userBody.Username)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		ok, err := u.es.postUser(req.Context(), *newUser)
		if ok && err == nil {
			util.WriteBackRaw(w, rawUser, http.StatusCreated)
			return
		}

		msg := fmt.Sprintf(`an error occurred while creating a user with "username"="%s": %v`, userBody.Username, err)
		log.Printf("%s: %s\n", logTag, msg)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *users) patchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		username, _, _ := req.BasicAuth()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Printf(fmt.Sprintf("%s: %s: %v\n", logTag, msg, err))
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		patch, err := userBody.GetPatch()
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// If user is trying to patch acls without providing categories.
		if patch["categories"] == nil && patch["acls"] != nil {
			// we need to fetch the user from elasticsearch before we make
			// a patch request in order to validate the acls that the user intends
			// to patch against the categories it already has.
			reqUser, err := u.es.getUser(req.Context(), username)
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while fetching user with username="%s"`, username)
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}

			acls, ok := patch["acls"].([]acl.ACL)
			if !ok {
				msg := fmt.Sprintf(`an error occurred while validating acls patch for user "%s"`, username)
				log.Printf("%s: unable to cast acls patch to []acl.ACL\n", logTag)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}

			if err := reqUser.ValidateACLs(acls...); err != nil {
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		raw, err := u.es.patchUser(req.Context(), username, patch)
		if err == nil {
			util.WriteBackRaw(w, raw, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *users) patchUserWithUsername() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username, ok := vars["username"]
		if !ok {
			util.WriteBackError(w, `can't patch user without a "username"`, http.StatusBadRequest)
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		patch, err := userBody.GetPatch()
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// If user is trying to patch acls without providing categories.
		if patch["categories"] == nil && patch["acls"] != nil {
			// we need to fetch the user object from elasticsearch before we make
			// a patch request in order to validate the acls that the user intends
			// to patch against the categories it already has.
			reqUser, err := u.es.getUser(req.Context(), username)
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while fetching user with username="%s"`, username)
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}

			acls, ok := patch["acls"].([]acl.ACL)
			if !ok {
				msg := fmt.Sprintf(`an error occurred while validating acls patch for user "%s"`, username)
				log.Printf("%s: unable to cast acl patch to []acl.ACL\n", logTag)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}

			if err := reqUser.ValidateACLs(acls...); err != nil {
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		raw, err := u.es.patchUser(req.Context(), username, patch)
		if err == nil {
			util.WriteBackRaw(w, raw, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *users) deleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		username, _, _ := req.BasicAuth()

		ok, err := u.es.deleteUser(req.Context(), username)
		if ok && err == nil {
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *users) deleteUserWithUsername() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username, ok := vars["username"]
		if !ok {
			util.WriteBackError(w, `can't delete a user without a "username"`, http.StatusBadRequest)
			return
		}

		ok, err := u.es.deleteUser(req.Context(), username)
		if ok && err == nil {
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}
