package users

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
)

func (u *Users) getUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userObj := ctx.Value(user.CtxKey)
		if userObj == nil {
			// redundant check, should be verified in authenticator
			userId, _, ok := r.BasicAuth()
			if !ok {
				util.WriteBackMessage(w, "credentials not provided", http.StatusUnauthorized)
				return
			}

			userObj, err := u.es.getRawUser(userId)
			if err != nil {
				msg := fmt.Sprintf("user with user_id=%s not found", userId)
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, userObj, http.StatusOK)
			return
		}

		raw, err := json.Marshal(userObj)
		if err != nil {
			msg := "error parsing the context user object"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (u *Users) putUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify if user creating a user is an admin, authenticator?
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "can't read body"
			log.Printf(fmt.Sprintf("%s: %s: %v", logTag, msg, err))
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		// TODO: Validate every necessary field is passed. Refer putPermissionsHandler
		var userObj user.User
		err = json.Unmarshal(body, &userObj)
		if err != nil {
			msg := "error parsing request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		ok, err := u.es.putUser(userObj)
		if ok && err == nil {
			util.WriteBackMessage(w, "successfully added user", http.StatusOK)
			return
		}

		msg := fmt.Sprintf("unable to store user with user_id=%s", userObj.UserId)
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *Users) patchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackMessage(w, "credentials not provided", http.StatusUnauthorized)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "can't read body"
			log.Printf(fmt.Sprintf("%s: %s: %v", logTag, msg, err))
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userObj user.User
		err = json.Unmarshal(body, &userObj)
		log.Printf("received fields: %v", userObj)
		if err != nil {
			msg := "error parsing request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		ok, err = u.es.patchUser(userId, userObj)
		if ok && err == nil {
			util.WriteBackMessage(w, "successfully updated user", http.StatusOK)
			return
		}

		msg := fmt.Sprintf("error updating user with user_id=%s", userId)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *Users) deleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// redundant check, should be verified in authenticator
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "credentials not provided", http.StatusUnauthorized)
			return
		}

		ok, err := u.es.deleteUser(userId)
		if ok && err == nil {
			util.WriteBackMessage(w, "successfully deleted user", http.StatusOK)
			return
		}

		msg := fmt.Sprintf("error deleting user with user_id=%s", userId)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}
