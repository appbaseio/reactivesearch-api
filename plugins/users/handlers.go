package users

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/gorilla/mux"
)

func (u *users) getUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// check the request context
		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser != nil {
			reqUser, ok := ctxUser.(*user.User)
			if !ok {
				msg := fmt.Sprintf("Cannot cast context user to *user.User")
				log.Printf("%s: %s\n", logTag, msg)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			rawUser, err := json.Marshal(*reqUser)
			if err != nil {
				msg := "Error parsing the context user object"
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, rawUser, http.StatusOK)
			return
		}

		// redundant check, should be verified in authenticator
		userID, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "Credentials not provided", http.StatusUnauthorized)
			return
		}

		// fetch the user from elasticsearch
		rawUser, err := u.es.getRawUser(userID)
		if err != nil {
			msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, userID)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
	}
}

func (u *users) getUserWithID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userID, ok := vars["user_id"]
		if !ok {
			util.WriteBackError(w, `Can't get a user without a "user_id"`, http.StatusBadRequest)
			return
		}

		rawUser, err := u.es.getRawUser(userID)
		if err != nil {
			msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, userID)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
	}
}

func (u *users) postUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "Can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		opts := []user.Options{
			user.SetEmail(userBody.Email),
		}
		if userBody.IsAdmin != nil {
			opts = append(opts, user.SetIsAdmin(userBody.IsAdmin))
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
		if userBody.UserId == "" {
			util.WriteBackError(w, `Can't create a user without a "user_id"`, http.StatusBadRequest)
			return
		}
		if userBody.Password == "" {
			util.WriteBackError(w, `Can't create a user without a "password"`, http.StatusBadRequest)
			return
		}
		newUser, err := user.New(userBody.UserId, userBody.Password, opts...)
		if err != nil {
			msg := fmt.Sprintf("Error constructing user object: %v", err)
			log.Printf("%s: %s\n", logTag, msg)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		rawUser, err := json.Marshal(*newUser)
		if err != nil {
			msg := fmt.Sprintf(`An error occurred while creating a user with "user_id"="%s"`, userBody.UserId)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		// TODO: check if user already exists
		ok, err := u.es.postUser(*newUser)
		if ok && err == nil {
			util.WriteBackRaw(w, rawUser, http.StatusCreated)
			return
		}

		msg := fmt.Sprintf(`An error occurred while creating a user with "user_id"="%s"`, userBody.UserId)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *users) patchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "Credentials not provided", http.StatusUnauthorized)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf(fmt.Sprintf("%s: %s: %v\n", logTag, msg, err))
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "Can't parse request body"
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

		raw, err := u.es.patchUser(userID, patch)
		if err == nil {
			util.WriteBackRaw(w, raw, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, userID)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *users) patchUserWithID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userID, ok := vars["user_id"]
		if !ok {
			util.WriteBackError(w, `Can't patch user without a "user_id"`, http.StatusBadRequest)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "Can't parse request body"
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

		raw, err := u.es.patchUser(userID, patch)
		if err == nil {
			util.WriteBackRaw(w, raw, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, userID)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *users) deleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// redundant check, should be verified in authenticator
		userID, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "Credentials not provided", http.StatusUnauthorized)
			return
		}

		ok, err := u.es.deleteUser(userID)
		if ok && err == nil {
			msg := fmt.Sprintf(`Successfully deleted user with "user_id"="%s"`, userID)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, userID)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *users) deleteUserWithID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userID, ok := vars["user_id"]
		if !ok {
			util.WriteBackError(w, `Can't delete a user without a "user_id"`, http.StatusBadRequest)
			return
		}

		ok, err := u.es.deleteUser(userID)
		if ok && err == nil {
			msg := fmt.Sprintf(`User with "user_id"="%s" deleted`, userID)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, userID)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}
