package permissions

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/gorilla/mux"
)

// getPermission fetches the permission from elasticsearch. If the request context
// already bears *permission.Permission then we simply return the marshaled context
// permission. However, authenticator authenticates the access for permissions endpoints
// against user.User and thus every time this handler is executed, we fetch the
// permission from the elasticsearch. An error on the side of elasticsearch client
// causes the handler to return http.StatusInternalServerError.
func (p *permissions) getPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]
		ctx := r.Context()

		// check the request context
		permissionCtx := ctx.Value(permission.CtxKey)
		if permissionCtx != nil {
			p := permissionCtx.(*permission.Permission)
			rawPermission, err := json.Marshal(*p)
			if err != nil {
				msg := "error parsing the context permissions object"
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, rawPermission, http.StatusOK)
		}

		// fetch the permission from elasticsearch
		rawPermission, err := p.es.getRawPermission(username)
		if err != nil {
			msg := fmt.Sprintf(`Permission with "username"="%s" not found`, username)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawPermission, http.StatusOK)
	}
}

// postPermission creates a new permission.Permission and indexes it in elastic search.
// The handler expects "user_id" in basic auth for the permission.Permission it intends
// to create and a request body that conforms to the permission.Permission struct. Omitted
// fields in the request body will assume default values. Invalid values passed explicitly
// in the request body will cause the handler to return http.StatusBadRequest. A raw/json
// permission is returned when a permission is successfully indexed in elasticsearch. An
// error on the side of elasticsearch client will cause the handler to return
// http.InternalServerError.
func (p *permissions) postPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// redundant check
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "Basic auth credentials not provided", http.StatusUnauthorized)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var obj permission.Permission
		err = json.Unmarshal(body, &obj)
		if err != nil {
			msg := "Can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		var opts []permission.Options
		if obj.UserId != "" {
			opts = append(opts, permission.SetUserId(obj.UserId))
		}
		if obj.Ops != nil {
			opts = append(opts, permission.SetOps(obj.Ops))
		}
		if obj.ACLs != nil {
			opts = append(opts, permission.SetACLs(obj.ACLs))
		}
		if obj.Limits != nil {
			opts = append(opts, permission.SetLimits(obj.Limits))
		}
		if obj.Indices != nil {
			opts = append(opts, permission.SetIndices(obj.Indices))
		}
		newPermission, err := permission.New(userId, opts...)
		if err != nil {
			msg := fmt.Sprintf("error constructing permission object: %v", err)
			log.Printf("%s: %s", logTag, msg)
			util.WriteBackError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		rawPermission, err := json.Marshal(*newPermission)
		if err != nil {
			log.Printf("%s: unable to marshal newPermission object: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ok, err = p.es.postPermission(*newPermission)
		if ok && err == nil {
			util.WriteBackRaw(w, rawPermission, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while creating a permission for "user_id"="%s"`, userId)
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}
}

// patchPermission modifies explicit fields in the indexed permission.Permission. The handler
// expects a request body that conforms to permission.Permission struct. The fields whose
// values are explicitly provided in the request body will only be overwritten. Invalid field
// values passed explicitly in the request body will cause the handler to return
// http.StatusBadRequest. However, an error on the side of elasticsearch client will cause
// the handler to return http.StatusInternalServerError.
func (p *permissions) patchPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var obj permission.Permission
		err = json.Unmarshal(body, &obj)
		if err != nil {
			msg := "Can't parse request body"
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

		ok, err := p.es.patchPermission(username, patch)
		if ok && err == nil {
			msg := fmt.Sprintf(`Successfully updated permission with "username"="%s"`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while updating permission with "username"="%s"`, username)
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

// deletePermission deletes the permission.Permission from elasticsearch. An error on
// the side of elasticsearch client will cause the handler to return http.InternalServerError.
func (p *permissions) deletePermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		ok, err := p.es.deletePermission(username)
		if ok && err == nil {
			msg := fmt.Sprintf(`Successfully deleted permission with "username"="%s"`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`Permission with "username"="%s" doesn't exist'`, username)
		log.Printf("%s: %s: %v", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

// getUserPermissions fetches all the permissions associated with user from elasticsearch.
// The handler expects "user_id" in basic auth for the permissions it intends from
// elasticsearch. An error on the side of elasticsearch client causes the handler to
// return http.StatusInternalServerError.
func (p *permissions) getUserPermissions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, _, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "Basic auth credentials not provided", http.StatusUnauthorized)
			return
		}

		raw, err := p.es.getUserPermissions(userId)
		if err != nil {
			msg := fmt.Sprintf(`A error occurred while fetching permissions for "user_id"="%s"`, userId)
			log.Printf("%s: %s: %v", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
