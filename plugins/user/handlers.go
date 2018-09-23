package user

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
)

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := ctx.Value(user.CtxKey)
	if u == nil {
		// redundant check, should be verified in authenticator
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackMessage(w, "credentials not provided", http.StatusUnauthorized)
			return
		}

		userObj, err := getRawUser(userId)
		if err != nil {
			msg := fmt.Sprintf("user with user_id=%s not found", userId)
			log.Printf("%s: %v\n", msg, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, userObj, http.StatusOK)
		return
	}

	raw, err := json.Marshal(u)
	if err != nil {
		msg := "error parsing the context user object"
		log.Printf("%s: %v\n", msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}

	util.WriteBackRaw(w, raw, http.StatusOK)
}

func putUserHandler(w http.ResponseWriter, r *http.Request) {
	// Verify if user creating a user is an admin, authenticator?
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := "can't read body"
		log.Printf(fmt.Sprintf("%s: %v", msg, err))
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}

	// TODO: Validate every necessary field is passed. Refer putPermissionsHandler
	var u user.User
	err = json.Unmarshal(body, &u)
	if err != nil {
		msg := "error parsing request body"
		log.Printf("%s: %v\n", msg, err)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}

	ok, err := putUser(u)
	if ok && err == nil {
		util.WriteBackMessage(w, "successfully added user", http.StatusOK)
		return
	}

	msg := fmt.Sprintf("unable to store user with user_id=%s", u.UserId)
	log.Printf("%s: %v", msg, err)
	util.WriteBackError(w, msg, http.StatusInternalServerError)
}

func patchUserHandler(w http.ResponseWriter, r *http.Request) {
	userId, _, ok := r.BasicAuth()
	if !ok {
		util.WriteBackMessage(w, "credentials not provided", http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := "can't read body"
		log.Printf(fmt.Sprintf("%s: %v", msg, err))
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}

	var u user.User
	err = json.Unmarshal(body, &u)
	log.Printf("received fields: %v", u)
	if err != nil {
		msg := "error parsing request body"
		log.Printf("%s: %v\n", msg, err)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}

	ok, err = patchUser(userId, u)
	if ok && err == nil {
		util.WriteBackMessage(w, "successfully updated user", http.StatusOK)
		return
	}

	msg := fmt.Sprintf("error updating user with user_id=%s", userId)
	log.Printf("%s: %v\n", msg, err)
	util.WriteBackError(w, msg, http.StatusInternalServerError)
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	// redundant check, should be verified in authenticator
	userId, _, ok := r.BasicAuth()
	if !ok {
		util.WriteBackError(w, "credentials not provided", http.StatusUnauthorized)
		return
	}

	ok, err := deleteUser(userId)
	if ok && err == nil {
		util.WriteBackMessage(w, "successfully deleted user", http.StatusOK)
		return
	}

	msg := fmt.Sprintf("error deleting user with user_id=%s", userId)
	log.Printf("%s: %v\n", msg, err)
	util.WriteBackError(w, msg, http.StatusInternalServerError)
}
