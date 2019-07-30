package permissions

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
)

func (p *permissions) getPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username := vars["username"]

		rawPermission, err := p.es.getRawPermission(req.Context(), username)
		if err != nil {
			msg := fmt.Sprintf(`permission with "username"="%s" not found`, username)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawPermission, http.StatusOK)
	}
}

func (p *permissions) postPermission(opts ...permission.Options) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		creator, _, _ := req.BasicAuth()
		reqUser, err := user.FromContext(req.Context())
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var permissionBody permission.Permission
		err = json.Unmarshal(body, &permissionBody)
		if err != nil {
			msg := "can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		if permissionBody.Owner != "" {
			opts = append(opts, permission.SetOwner(permissionBody.Owner))
		}
		if permissionBody.Ops != nil {
			opts = append(opts, permission.SetOps(permissionBody.Ops))
		}
		if permissionBody.Role != "" {
			opts = append(opts, permission.SetRole(permissionBody.Role))
		}
		if permissionBody.Categories != nil {
			opts = append(opts, permission.SetCategories(permissionBody.Categories))
		}
		if permissionBody.ACLs != nil {
			opts = append(opts, permission.SetACLs(permissionBody.ACLs))
		}
		if permissionBody.Sources != nil {
			opts = append(opts, permission.SetSources(permissionBody.Sources))
		}
		if permissionBody.Referers != nil {
			opts = append(opts, permission.SetReferers(permissionBody.Referers))
		}
		if permissionBody.Indices != nil {
			opts = append(opts, permission.SetIndices(permissionBody.Indices))
		}
		if permissionBody.Limits != nil {
			opts = append(opts, permission.SetLimits(permissionBody.Limits))
		}
		if permissionBody.Description != "" {
			opts = append(opts, permission.SetDescription(permissionBody.Description))
		}
		if permissionBody.TTL != 0 {
			opts = append(opts, permission.SetTTL(permissionBody.TTL))
		}

		var newPermission *permission.Permission
		if *reqUser.IsAdmin {
			newPermission, err = permission.NewAdmin(creator, opts...)
		} else {
			newPermission, err = permission.New(creator, opts...)
		}
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		rawPermission, err := json.Marshal(*newPermission)
		if err != nil {
			msg := fmt.Sprintf(`an error occurred while creating permission for "creator"="%s"`, creator)
			log.Printf("%s: unable to marshal newPermission object: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		if newPermission.Role != "" {
			var roleExists bool
			roleExists, err = p.es.checkRoleExists(req.Context(), newPermission.Role)
			if roleExists {
				msg := fmt.Sprintf(`permission with role=%s already exists`, newPermission.Role)
				log.Printf("%s: %s\n", logTag, msg)
				util.WriteBackError(w, msg, http.StatusBadRequest)
				return
			}
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while creating permission for role=%s`, creator, newPermission.Role)
				log.Printf("%s: unable to check if role=%s exists: %v\n", logTag, newPermission.Role)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
		}

		ok, err := p.es.postPermission(req.Context(), *newPermission)
		if ok && err == nil {
			util.WriteBackRaw(w, rawPermission, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`an error occurred while creating permission for "creator"="%s"`, creator)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}
}

func (p *permissions) patchPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username := vars["username"]

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var obj permission.Permission
		err = json.Unmarshal(body, &obj)
		if err != nil {
			msg := "can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var perMap map[string]interface{}
		err = json.Unmarshal(body, &perMap)
		if err != nil {
			msg := "can't parse request body"
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}
		_, roleExistsInPatch := perMap["role"]

		patch, err := obj.GetPatch(roleExistsInPatch)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// If user is trying to patch acls without providing categories.
		if patch["categories"] == nil && patch["acls"] != nil {
			// we need to fetch the permission from elasticsearch before we make
			// a patch request in order to validate the acls that the user intends
			// to patch against the categories it already has.
			reqPermission, err := p.es.getPermission(req.Context(), username)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			acls, ok := patch["acls"].([]acl.ACL)
			if !ok {
				msg := fmt.Sprintf(`an error occurred while validating categories patch for user "%s"`, username)
				log.Printf("%s: unable to cast categories patch to []acl.ACL\n", logTag)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}

			if err := reqPermission.ValidateACLs(acls...); err != nil {
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		if roleExistsInPatch && patch["role"] != "" {
			var roleExistsInES bool
			roleExistsInES, err = p.es.checkRoleExists(req.Context(), obj.Role)
			if roleExistsInES {
				msg := fmt.Sprintf(`permission with role=%s already exists`, obj.Role)
				log.Printf("%s: %s\n", logTag, msg)
				util.WriteBackError(w, msg, http.StatusBadRequest)
				return
			}
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while creating permission for role=%s`, obj.Role)
				log.Printf("%s: unable to check if role=%s exists: %v\n", logTag, obj.Role)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
		}

		raw, err := p.es.patchPermission(req.Context(), username, patch)
		if err == nil {
			util.WriteBackRaw(w, raw, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`permission with "username"="%s" not found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (p *permissions) deletePermission() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username := vars["username"]

		ok, err := p.es.deletePermission(req.Context(), username)
		if ok && err == nil {
			msg := fmt.Sprintf(`permission with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`permission with "username"="%s" not found`, username)
		log.Printf("%s: %s: %v\n", logTag, msg, err)
		util.WriteBackError(w, msg, http.StatusNotFound)
	}
}

func (p *permissions) getUserPermissions() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		owner, _, _ := req.BasicAuth()

		raw, err := p.es.getRawOwnerPermissions(req.Context(), owner)
		if err != nil {
			msg := fmt.Sprintf(`an error occurred while fetching permissions for "owner"="%s"`, owner)
			log.Printf("%s: %s: %v\n", logTag, msg, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (p *permissions) role() http.HandlerFunc {
	return func (w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		role := vars["name"]

		var raw []byte
		var perm permission.Permission
		if req.Method != http.MethodPost {
			var err error
			raw, err = p.es.getRawRolePermission(req.Context(), role)
			if raw == nil || err != nil {
				msg := fmt.Sprintf(`an error occurred while fetching permissions for role=%s`, role)
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusNotFound)
				return
			}
			err = json.Unmarshal(raw, &perm)
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while fetching permissions for role=%s`, role)
				log.Printf("%s: %s: %v\n", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return			
			}
		}

		switch req.Method {
		case http.MethodGet:
			util.WriteBackRaw(w, raw, http.StatusOK)
		case http.MethodPost:
			p.postPermission(permission.SetRole(role))(w, req)
			return
		case http.MethodPatch:
			http.Redirect(w, req, "/_permission/" + perm.Username, 308)
			return
		case http.MethodDelete:
			http.Redirect(w, req, "/_permission/" + perm.Username, 308)
			return
		}
	}
}
