package permissions

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

func (p *Permissions) getPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		// TODO: Is it crucial to store permissions in request context?
		ctx := r.Context()
		permissionObj := ctx.Value(permission.CtxKey)
		if permissionObj == nil {
			permissionObj, err := p.es.getRawPermissions(username)
			if err != nil {
				msg := fmt.Sprintf("cannot fetch permissions for username=%s", username)
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusNotFound)
				return
			}
			util.WriteBackRaw(w, permissionObj, http.StatusOK)
			return
		}

		raw, err := json.Marshal(permissionObj)
		if err != nil {
			msg := "error parsing the context permissions object"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (p *Permissions) putPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "credentials not provided", http.StatusUnauthorized)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "can't read body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var permissionObj permission.Permission
		err = json.Unmarshal(body, &permissionObj)
		if err != nil {
			msg := "error parsing request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		builder := permission.NewBuilder(userId).Permission(permissionObj)
		if permissionObj.Creator == "" {
			// TODO: Authenticate the creator of permission
			msg := "empty creator passed in body"
			log.Printf("%s\n", msg)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}
		builder = builder.Creator(permissionObj.Creator)

		if permissionObj.Op == op.Noop {
			msg := "invalid operation passed in body"
			log.Printf("%s: %s: %v", logTag, msg, permissionObj.Op)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}
		builder = builder.Op(permissionObj.Op)

		if permissionObj.ACL == nil || len(permissionObj.ACL) <= 0 {
			msg := "empty set of acls passed in body"
			log.Printf("%s: %s\n", logTag, msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		builder = builder.ACL(permissionObj.ACL)

		if permissionObj.Indices == nil {
			msg := "empty set of indices passed in body\n"
			log.Printf("%s: %s\n", logTag, msg)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}
		builder = builder.Indices(permissionObj.Indices)
		permission := builder.Build()
		raw, _ := json.Marshal(permission)

		ok, err = p.es.putPermission(permission)
		if ok && err == nil {
			util.WriteBackRaw(w, raw, http.StatusOK)
			return
		}

		msg := fmt.Sprintf("unable to create permission for user_id=%s: %v\n", userId, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}
}

func (p *Permissions) patchPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "can't read body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var permissionObj permission.Permission
		err = json.Unmarshal(body, &permissionObj)
		if err != nil {
			msg := "error parsing request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		ok, err := p.es.patchPermission(username, permissionObj)
		if ok && err == nil {
			util.WriteBackMessage(w, "successfully updated permission", http.StatusOK)
			return
		}

		msg := fmt.Sprintf("error updating permission for user_id=%s")
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (p *Permissions) deletePermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		ok, err := p.es.deletePermission(username)
		if ok && err == nil {
			util.WriteBackMessage(w, "successfully deleted permission", http.StatusOK)
			return
		}

		msg := fmt.Sprintf("error deleting permission for username=%s", username)
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}
