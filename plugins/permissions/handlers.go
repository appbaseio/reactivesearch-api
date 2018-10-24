package permissions

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/gorilla/mux"
)

func (p *permissions) getPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		rawPermission, err := p.es.getRawPermission(username)
		if err != nil {
			msg := fmt.Sprintf(`Permission with "username"="%s" Not Found`, username)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawPermission, http.StatusOK)
	}
}

func (p *permissions) postPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creator, _, _ := r.BasicAuth()
		reqUser, err := user.FromContext(r.Context())
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			msg := "Can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var permissionBody permission.Permission
		err = json.Unmarshal(body, &permissionBody)
		if err != nil {
			msg := "Can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var opts []permission.Options
		if permissionBody.Owner != "" {
			opts = append(opts, permission.SetOwner(permissionBody.Owner))
		}
		if permissionBody.Ops != nil {
			opts = append(opts, permission.SetOps(permissionBody.Ops))
		}
		if permissionBody.ACLs != nil {
			opts = append(opts, permission.SetACLs(permissionBody.ACLs))
		}
		if permissionBody.Limits != nil {
			opts = append(opts, permission.SetLimits(permissionBody.Limits))
		}
		if permissionBody.Indices != nil {
			opts = append(opts, permission.SetIndices(permissionBody.Indices))
		}

		var newPermission *permission.Permission
		if *reqUser.IsAdmin {
			newPermission, err = permission.NewAdmin(creator, opts...)
		} else {
			newPermission, err = permission.New(creator, opts...)
		}
		if err != nil {
			msg := fmt.Sprintf("Error constructing permission object: %v", err)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		rawPermission, err := json.Marshal(*newPermission)
		if err != nil {
			msg := fmt.Sprintf(`An error occurred while creating a permission for "creator"="%s"`, creator)
			log.Printf("%s: unable to marshal newPermission object: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		ok, err := p.es.postPermission(*newPermission)
		if ok && err == nil {
			util.WriteBackRaw(w, rawPermission, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`An error occurred while creating a permission for "creator"="%s"`, creator)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}
}

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
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		raw, err := p.es.patchPermission(username, patch)
		if err == nil {
			util.WriteBackRaw(w, raw, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`Permission with "username"="%s" Not Found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
	}
}

func (p *permissions) deletePermission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		username := vars["username"]

		ok, err := p.es.deletePermission(username)
		if ok && err == nil {
			msg := fmt.Sprintf(`Permission with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`Permission with "username"="%s" Not Found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (p *permissions) getUserPermissions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owner, _, _ := r.BasicAuth()

		raw, err := p.es.getOwnerPermissions(owner)
		if err != nil {
			msg := fmt.Sprintf(`An error occurred while fetching permissions for "owner"="%s"`, owner)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
