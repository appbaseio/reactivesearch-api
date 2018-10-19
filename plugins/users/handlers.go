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

// getUser fetches the user from elasticsearch. The handler expects "user_id" in basic
// auth for the user.User the request intends to fetch from the elasticsearch. If the
// request context already bears *user.User, then we simply return the
// marshaled context user. And since the current authenticator requires a user.User for
// authentication, the request context must always have a stored pointer to the
// authenticated *user.User. An error on the side of elasticsearch client causes the
// handler to return http.StatusInternalServerError.
func (u *users) getUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// check the request context
		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser != nil {
			u := ctxUser.(*user.User)
			rawUser, err := json.Marshal(*u)
			if err != nil {
				msg := "error parsing the context user object"
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, rawUser, http.StatusOK)
			return
		}

		// redundant check, should be verified in authenticator
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackMessage(w, "Basic auth credentials not provided", http.StatusUnauthorized)
			return
		}

		// fetch the user from elasticsearch
		rawUser, err := u.es.getRawUser(userId)
		if err != nil {
			msg := fmt.Sprintf(`User with "user_id"="%s" not found`, userId)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
	}
}

func (u *users) getUserWithId() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userId, ok := vars["user_id"]
		if !ok {
			util.WriteBackError(w, `Can't get a user without "user_id"`, http.StatusBadRequest)
			return
		}

		rawUser, err := u.es.getRawUser(userId)
		if err != nil {
			msg := fmt.Sprintf(`User with "user_id"="%s" not found`, userId)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
	}
}

// postUser creates a new user.User and indexes it in elasticsearch. The handler expects
// "user_id" and "password" in basic auth for the user.User it intends to create and a
// request body that conforms to the user.User struct. Omitted fields in the request body
// will assume default values. Invalid values passed explicitly in the request body
// will cause the handler to return http.StatusBadRequest. A raw/json user is returned
// when a user is successfully indexed in elasticsearch. An error on the side of
// elasticsearch client will cause the handler to return http.InternalServerError.
func (u *users) postUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var obj user.User
		err = json.Unmarshal(body, &obj)
		if err != nil {
			msg := "Can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		opts := []user.Options{
			user.SetEmail(obj.Email),
		}
		if obj.IsAdmin != nil {
			opts = append(opts, user.SetIsAdmin(obj.IsAdmin))
		}
		if obj.ACLs != nil {
			opts = append(opts, user.SetACLs(obj.ACLs))
		}
		if obj.Ops != nil {
			opts = append(opts, user.SetOps(obj.Ops))
		}
		if obj.Indices != nil {
			opts = append(opts, user.SetIndices(obj.Indices))
		}
		if obj.UserId == "" {
			util.WriteBackError(w, `Can't create a user without "user_id"`, http.StatusBadRequest)
			return
		}
		if obj.Password == "" {
			util.WriteBackError(w, `Can't create a user without "password"`, http.StatusBadRequest)
			return
		}
		newUser, err := user.New(obj.UserId, obj.Password, opts...)
		if err != nil {
			msg := fmt.Sprintf("error constructing user object: %v", err)
			log.Printf("%s: %s", logTag, msg)
			util.WriteBackError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		rawUser, err := json.Marshal(*newUser)
		if err != nil {
			log.Printf("%s: unable to marshal newUser object: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// TODO: check if user already exists
		ok, err := u.es.postUser(*newUser)
		if ok && err == nil {
			util.WriteBackRaw(w, rawUser, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while creating a user with "user_id"="%s"`, obj.UserId)
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

// patchUser modifies explicit fields in the indexed user.User. The handler expects
// "user_id" and "password" in basic auth for the user.User the request intends to
// modify and a request body that conforms to the user.User struct; unless otherwise
// it returns http.StatusBadRequest. The fields whose values are explicitly provided
// in the request body will only be overwritten. Invalid field values passed explicitly
// in the request body will cause the handler to return http.StatusBadRequest. However,
// an error on the side of elasticsearch client will cause the handler to return
// http.StatusInternalServerError.
func (u *users) patchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// redundant check
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackMessage(w, "Basic auth credentials not provided", http.StatusUnauthorized)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf(fmt.Sprintf("%s: %s: %v", logTag, msg, err))
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

		ok, err = u.es.patchUser(userId, patch)
		if ok && err == nil {
			msg := fmt.Sprintf(`Successfully updated user with "user_id"="%s"`, userId)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while updating user with "user_id"="%s"`, userId)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *users) patchUserWithId() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userId, ok := vars["user_id"]
		if !ok {
			util.WriteBackError(w, `Can't patch user without "user_id"`, http.StatusBadRequest)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf(fmt.Sprintf("%s: %s: %v", logTag, msg, err))
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

		ok, err = u.es.patchUser(userId, patch)
		if ok && err == nil {
			msg := fmt.Sprintf(`Successfully updated user with "user_id"="%s"`, userId)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while updating user with "user_id"="%s"`, userId)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

// deleteUser deletes the user.User from elasticsearch. THe handler expects user_id
// and password for the user.User the request intends to delete. An error on the side
// of elasticsearch client will cause the handler to return http.InternalServerError.
func (u *users) deleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// redundant check, should be verified in authenticator
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "Basic auth credentials not provided", http.StatusUnauthorized)
			return
		}

		ok, err := u.es.deleteUser(userId)
		if ok && err == nil {
			msg := fmt.Sprintf(`Successfully deleted user with "user_id"="%s"`, userId)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while deleting user with "user_id"="%s"`, userId)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *users) deleteUserWithId() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userId, ok := vars["user_id"]
		if !ok {
			util.WriteBackError(w, `Can't delete a user without a "user_id"`, http.StatusBadRequest)
			return
		}

		ok, err := u.es.deleteUser(userId)
		if ok && err == nil {
			msg := fmt.Sprintf(`Successfully deleted user with "user_id"="%s"`, userId)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while deleting user with "user_id"="%s"`, userId)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}
