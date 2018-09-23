package permission

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/gorilla/mux"
)

func getPermissionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	// TODO: Is it crucial to store permissions in request context?
	ctx := r.Context()
	p := ctx.Value(permission.CtxKey)
	if p == nil {
		permsObj, err := getRawPermissions(username)
		if err != nil {
			msg := fmt.Sprintf("cannot fetch permissions for username=%s", username)
			log.Printf("%s: %v\n", msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, permsObj, http.StatusOK)
		return
	}

	raw, err := json.Marshal(p)
	if err != nil {
		msg := "error parsing the context permissions object"
		log.Printf("%s: %v\n", msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}

	util.WriteBackRaw(w, raw, http.StatusOK)
}

func putPermissionHandler(w http.ResponseWriter, r *http.Request) {
	userId, _, ok := r.BasicAuth()
	if !ok {
		util.WriteBackError(w, "credentials not provided", http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := "can't read body"
		log.Printf("%s: %v\n", msg, err)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}

	var p permission.Permission
	err = json.Unmarshal(body, &p)
	if err != nil {
		msg := "error parsing request body"
		log.Printf("%s: %v\n", msg, err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	builder := permission.NewBuilder(userId).Permission(p)
	if p.Creator == "" {
		// TODO: Authenticate the creator of permission
		msg := "empty creator passed in body"
		log.Printf("%s\n", msg)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}
	builder = builder.Creator(p.Creator)

	if p.Op == op.Noop {
		msg := "invalid operation passed in body"
		log.Printf("%s: %v", msg, p.Op)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}
	builder = builder.Op(p.Op)

	if p.ACL == nil || len(p.ACL) <= 0 {
		msg := "empty set of acls passed in body"
		log.Printf("%s\n", msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	builder = builder.ACL(p.ACL)

	if p.Indices == nil {
		msg := "empty set of indices passed in body\n"
		log.Printf("%s\n", msg)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}
	builder = builder.Indices(p.Indices)
	perm := builder.Build()
	raw, _ := json.Marshal(perm)

	ok, err = putPermission(perm)
	if ok && err == nil {
		util.WriteBackRaw(w, raw, http.StatusOK)
		return
	}

	msg := fmt.Sprintf("unable to create permission for user_id=%s: %v\n", userId, err)
	util.WriteBackError(w, msg, http.StatusInternalServerError)
	return
}

func patchPermissionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := "can't read body"
		log.Printf("%s: %v\n", msg, err)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}

	var p permission.Permission
	err = json.Unmarshal(body, &p)
	if err != nil {
		msg := "error parsing request body"
		log.Printf("%s: %v\n", msg, err)
		util.WriteBackError(w, msg, http.StatusBadRequest)
		return
	}

	ok, err := patchPermission(username, p)
	if ok && err == nil {
		util.WriteBackMessage(w, "successfully updated permission", http.StatusOK)
		return
	}

	msg := fmt.Sprintf("error updating permission for user_id=%s")
	log.Printf("%s: %v", msg, err)
	util.WriteBackError(w, msg, http.StatusInternalServerError)
}

func deletePermissionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	ok, err := deletePermission(username)
	if ok && err == nil {
		util.WriteBackMessage(w, "successfully deleted permission", http.StatusOK)
		return
	}

	msg := fmt.Sprintf("error deleting permission for username=%s", username)
	log.Printf("%s: %v", msg, err)
	util.WriteBackError(w, msg, http.StatusInternalServerError)
}
