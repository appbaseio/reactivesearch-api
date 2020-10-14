package users

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/plugins/auth"
	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

func (u *Users) getUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		username, _, _ := req.BasicAuth()

		// check the request context
		if reqUser, err := user.FromContext(ctx); err == nil {
			rawUser, err := json.Marshal(*reqUser)
			if err != nil {
				msg := "error parsing the context user object"
				log.Errorln(logTag, ":", msg, ":", err)
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
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
		return
	}
}

func (u *Users) getUserWithUsername() http.HandlerFunc {
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
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawUser, http.StatusOK)
	}
}

func (u *Users) postUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			const msg = "can't read request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "can't parse request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		opts := []user.Options{
			user.SetEmail(userBody.Email),
		}
		if userBody.IsAdmin != nil {
			opts = append(opts, user.SetIsAdmin(*userBody.IsAdmin))
		}
		if userBody.AllowedActions != nil {
			opts = append(opts, user.SetAllowedActions(*userBody.AllowedActions))
		}
		if userBody.ACLs != nil {
			opts = append(opts, user.SetACLs(userBody.ACLs))
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
		// If user is not an admin then at least one action must present
		if userBody.IsAdmin == nil || !*userBody.IsAdmin {
			if userBody.AllowedActions == nil || len(*userBody.AllowedActions) == 0 {
				util.WriteBackError(w, `user "allowed_actions" shouldn't be empty for non-admin users`, http.StatusBadRequest)
				return
			}
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userBody.Password), bcrypt.DefaultCost)
		if err != nil {
			msg := fmt.Sprintf("an error occurred while hashing password: %v", userBody.Password)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		var newUser *user.User
		if userBody.IsAdmin != nil && *userBody.IsAdmin {
			newUser, err = user.NewAdmin(userBody.Username, string(hashedPassword), opts...)
		} else {
			newUser, err = user.New(userBody.Username, string(hashedPassword), opts...)
		}

		if err != nil {
			msg := fmt.Sprintf("an error occurred while creating user: %v", err)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		newUser.PasswordHashType = "bcrypt"

		rawUser, err := json.Marshal(*newUser)
		if err != nil {
			msg := fmt.Sprintf(`an error occurred while creating a user with "username"="%s"`, userBody.Username)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		ok, err := u.es.postUser(req.Context(), *newUser)
		if ok && err == nil {
			util.WriteBackRaw(w, rawUser, http.StatusCreated)
			return
		}

		msg := fmt.Sprintf(`an error occurred while creating a user with "username"="%s": %v`, userBody.Username, err)
		log.Println(logTag, ":", msg)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (u *Users) patchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		username, _, _ := req.BasicAuth()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "can't parse request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		patch, err := userBody.GetPatch()
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// If user is trying to update the password then store the hashed password
		if patch["password"] != nil {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userBody.Password), bcrypt.DefaultCost)
			if err != nil {
				msg := fmt.Sprintf("an error occurred while hashing password: %v", userBody.Password)
				log.Errorln(logTag, ":", msg, ":", err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			patch["password"] = string(hashedPassword)
		}

		_, err2 := u.es.patchUser(req.Context(), username, patch)
		if err2 == nil {
			// Clear username record from the cache
			auth.ClearPassword(username)
			// Clear user record from the user cache
			auth.RemoveCredentialFromCache(username)
			util.WriteBackMessage(w, "User is updated successfully", http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *Users) patchUserWithUsername() http.HandlerFunc {
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
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var userBody user.User
		err = json.Unmarshal(body, &userBody)
		if err != nil {
			msg := "can't parse request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		patch, err := userBody.GetPatch()
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// If user is trying to update the password then store the hashed password
		if patch["password"] != nil {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userBody.Password), bcrypt.DefaultCost)
			if err != nil {
				msg := fmt.Sprintf("an error occurred while hashing password: %v", userBody.Password)
				log.Errorln(logTag, ":", msg, ":", err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			patch["password"] = string(hashedPassword)
		}

		_, err2 := u.es.patchUser(req.Context(), username, patch)
		if err2 == nil {
			// Clear username record from the cache
			auth.ClearPassword(username)
			// Clear user record from the user cache
			auth.RemoveCredentialFromCache(username)
			util.WriteBackMessage(w, "User is updated successfully", http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *Users) deleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		username, _, _ := req.BasicAuth()

		ok, err := u.es.deleteUser(req.Context(), username)
		if ok && err == nil {
			// Clear username record from the cache
			auth.ClearPassword(username)
			// Clear user record from the user cache
			auth.RemoveCredentialFromCache(username)
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *Users) deleteUserWithUsername() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username, ok := vars["username"]
		if !ok {
			util.WriteBackError(w, `can't delete a user without a "username"`, http.StatusBadRequest)
			return
		}

		ok, err := u.es.deleteUser(req.Context(), username)
		if ok && err == nil {
			// Clear username record from the cache
			auth.ClearPassword(username)
			// Clear user record from the user cache
			auth.RemoveCredentialFromCache(username)
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (u *Users) getAllUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		raw, err := u.es.getRawUsers(req.Context())
		if err != nil {
			msg := `an error occurred while fetching users`
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
