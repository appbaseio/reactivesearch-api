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

		obj := ctx.Value(user.CtxKey)
		if obj == nil {
			// redundant check, should be verified in authenticator
			userId, _, ok := r.BasicAuth()
			if !ok {
				util.WriteBackMessage(w, "credentials not provided", http.StatusUnauthorized)
				return
			}

			rawUser, err := u.es.getRawUser(userId)
			if err != nil {
				msg := fmt.Sprintf("user with user_id=%s not found", userId)
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, rawUser, http.StatusOK)
			return
		}

		rawUser, err := json.Marshal(obj)
		if err != nil {
			msg := "error parsing the context user object"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, rawUser, http.StatusOK)
	}
}

func (u *Users) putUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// redundant check
		userId, password, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "credentials not provided", http.StatusUnauthorized)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "can't read body"
			log.Printf(fmt.Sprintf("%s: %s: %v", logTag, msg, err))
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var obj user.User
		err = json.Unmarshal(body, &obj)
		if err != nil {
			msg := "error parsing request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		opts := []user.Options{
			user.Email(obj.Email),
		}
		if obj.IsAdmin != nil {
			opts = append(opts, user.IsAdmin(obj.IsAdmin))
		}
		if obj.ACLs != nil {
			opts = append(opts, user.ACLs(obj.ACLs))
		}
		if obj.Ops != nil {
			opts = append(opts, user.Ops(obj.Ops))
		}
		if obj.Indices != nil {
			opts = append(opts, user.Indices(obj.Indices))
		}
		newUser, err := user.New(userId, password, opts...)
		if err != nil {
			log.Printf("%s: error constructing user object: %v", logTag, err)
			util.WriteBackError(w, "Unable to create user", http.StatusInternalServerError)
			return
		}

		rawUser, err := json.Marshal(*newUser)
		if err != nil {
			log.Printf("%s: unable to marshal newUser object: %v", logTag, err)
			util.WriteBackMessage(w, "Unable to create user", http.StatusInternalServerError)
			return
		}

		ok, err = u.es.putUser(*newUser)
		if ok && err == nil {
			util.WriteBackRaw(w, rawUser, http.StatusOK)
			return
		}

		msg := fmt.Sprintf("unable to store user with user_id=%s", obj.UserId)
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *Users) patchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// redundant check
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

		var obj user.User
		err = json.Unmarshal(body, &obj)
		log.Printf("received fields: %v", obj)
		if err != nil {
			msg := "error parsing request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		patch, err := obj.GetPatch()
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		ok, err = u.es.patchUser(userId, patch)
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
