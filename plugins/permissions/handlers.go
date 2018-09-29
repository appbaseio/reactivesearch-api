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

func (p *Permissions) getPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]
		ctx := r.Context()

		// TODO: Is it crucial to store permissions in request context, caching makes more sense?
		var err error
		obj := ctx.Value(permission.CtxKey)
		if obj == nil {
			obj, err = p.es.getRawPermission(username)
			if err != nil {
				msg := fmt.Sprintf("cannot fetch permissions for username=%s", username)
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusNotFound)
				return
			}
			util.WriteBackRaw(w, obj.([]byte), http.StatusOK)
			return
		}

		raw, err := json.Marshal(obj)
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

		var obj permission.Permission
		err = json.Unmarshal(body, &obj)
		if err != nil {
			msg := "error parsing request body"
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
		newPermission, err := permission.New(userId, opts...)
		if err != nil {
			log.Printf("%s: error constructing permission object: %v", logTag, err)
			util.WriteBackError(w, "Unable to create permission", http.StatusInternalServerError)
			return
		}

		rawPermission, err := json.Marshal(*newPermission)
		if err != nil {
			log.Printf("%s: unable to marshal newPermission object: %v", logTag, err)
			util.WriteBackMessage(w, "Unable to create permission", http.StatusInternalServerError)
			return
		}

		ok, err = p.es.putPermission(*newPermission)
		if ok && err == nil {
			util.WriteBackRaw(w, rawPermission, http.StatusOK)
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

		var obj permission.Permission
		err = json.Unmarshal(body, &obj)
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

		ok, err := p.es.patchPermission(username, patch)
		if ok && err == nil {
			util.WriteBackMessage(w, "successfully updated permission", http.StatusOK)
			return
		}

		msg := fmt.Sprintf("error updating permission for username=%s", username)
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
