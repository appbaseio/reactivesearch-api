package permissions

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gorilla/mux"
)

func (p *permissions) getPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username := vars["username"]

		rawPermission, err := p.es.getRawPermission(req.Context(), username)
		if err != nil {
			msg := fmt.Sprintf(`permission with "username"="%s" not found`, username)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawPermission, http.StatusOK)
	}
}

func (p *permissions) postPermission(opts ...permission.Options) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		creator, _, _ := req.BasicAuth()
		permissionOptions := []permission.Options{}
		// Copy the opts
		for _, v := range opts {
			permissionOptions = append(permissionOptions, v)
		}
		reqUser, err := user.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var permissionBody permission.Permission
		err = json.Unmarshal(body, &permissionBody)
		if err != nil {
			msg := "can't parse request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		if permissionBody.Owner != "" {
			permissionOptions = append(permissionOptions, permission.SetOwner(permissionBody.Owner))
		}
		if permissionBody.Ops != nil {
			permissionOptions = append(permissionOptions, permission.SetOps(permissionBody.Ops))
		}
		if permissionBody.Role != "" {
			permissionOptions = append(permissionOptions, permission.SetRole(permissionBody.Role))
		}
		if permissionBody.Categories != nil {
			permissionOptions = append(permissionOptions, permission.SetCategories(permissionBody.Categories))
		}
		if permissionBody.ACLs != nil {
			permissionOptions = append(permissionOptions, permission.SetACLs(permissionBody.ACLs))
		}
		if permissionBody.Sources != nil {
			permissionOptions = append(permissionOptions, permission.SetSources(permissionBody.Sources))
		}
		if permissionBody.Referers != nil {
			permissionOptions = append(permissionOptions, permission.SetReferers(permissionBody.Referers))
		}
		if permissionBody.Includes != nil {
			permissionOptions = append(permissionOptions, permission.SetIncludes(permissionBody.Includes))
		}
		if permissionBody.Excludes != nil {
			permissionOptions = append(permissionOptions, permission.SetExcludes(permissionBody.Excludes))
		}
		if permissionBody.Indices != nil {
			permissionOptions = append(permissionOptions, permission.SetIndices(permissionBody.Indices))
		}
		if permissionBody.Limits != nil {
			permissionOptions = append(permissionOptions, permission.SetLimits(permissionBody.Limits, *reqUser.IsAdmin))
		}
		if permissionBody.Description != "" {
			permissionOptions = append(permissionOptions, permission.SetDescription(permissionBody.Description))
		}
		if permissionBody.ReactiveSearchConfig != nil {
			permissionOptions = append(permissionOptions, permission.SetReactivesearchConfig(*permissionBody.ReactiveSearchConfig))
		}
		if permissionBody.TTL != 0 {
			permissionOptions = append(permissionOptions, permission.SetTTL(permissionBody.TTL))
		}

		var newPermission *permission.Permission
		if *reqUser.IsAdmin {
			newPermission, err = permission.NewAdmin(creator, permissionOptions...)
		} else {
			newPermission, err = permission.New(creator, permissionOptions...)
		}
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		rawPermission, err := json.Marshal(*newPermission)
		if err != nil {
			msg := fmt.Sprintf(`an error occurred while creating permission for "creator"="%s"`, creator)
			log.Errorln(logTag, ": unable to marshal newPermission object", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		if newPermission.Role != "" {
			var roleExists bool
			roleExists, err = p.es.checkRoleExists(req.Context(), newPermission.Role)
			if roleExists {
				msg := fmt.Sprintf(`permission with role=%s already exists`, newPermission.Role)
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, msg, http.StatusBadRequest)
				return
			}
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while creating permission for role=%s`, newPermission.Role)
				log.Errorln(logTag, ": unable to check if role=", newPermission.Role, "exists:", err)
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
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}
}

func (p *permissions) patchPermission() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username := vars["username"]
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// delete user details locally
			auth.ClearLocalUser(username)
			util.WriteBackMessage(w, "permission is updated successfully", http.StatusOK)
			return
		}
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			msg := "can't read request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var obj permission.Permission
		err = json.Unmarshal(body, &obj)
		if err != nil {
			msg := "can't parse request body"
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		var perMap map[string]interface{}
		err = json.Unmarshal(body, &perMap)
		if err != nil {
			msg := "can't parse request body"
			log.Errorln(logTag, ": ", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}
		_, roleExistsInPatch := perMap["role"]

		patch, err := obj.GetPatch(roleExistsInPatch)
		if err != nil {
			log.Errorln(logTag, ":", err)
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
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			acls, ok := patch["acls"].([]acl.ACL)
			if !ok {
				msg := fmt.Sprintf(`an error occurred while validating categories patch for user "%s"`, username)
				log.Println(logTag, ": unable to cast categories patch to []acl.ACL")
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
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, msg, http.StatusBadRequest)
				return
			}
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while creating permission for role=%s`, obj.Role)
				log.Errorln(logTag, ": unable to check if role=", obj.Role, "exists")
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
		}

		_, err2 := p.es.patchPermission(req.Context(), username, patch)
		if err2 == nil {
			// Only update local state when proxy API has not been called
			// If proxy API would get called then it would automatically update the
			// state for all machines
			// Updating the local state again can cause insconsistency issues
			if util.ShouldProxyToACCAPI() {
				// Invoke ACCAPI
				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodPatch,
					URL:    "/_permission/" + username,
					Body:   nil,
				})
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				// Failed to update all nodes, return error response
				if res != nil {
					log.Errorln(logTag, ":", "error encountered updating permission")
					bodyBytes, err := ioutil.ReadAll(res.Body)
					if err != nil {
						log.Errorln(logTag, ":", err)
						util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
						return
					}
					util.WriteBackRaw(w, bodyBytes, res.StatusCode)
					return
				}
			} else {
				// clear user details locally
				auth.ClearLocalUser(username)
			}
			util.WriteBackMessage(w, "permission is updated successfully", http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`permission with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
		return
	}
}

func (p *permissions) deletePermission() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		username := vars["username"]
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// delete user details locally
			auth.ClearLocalUser(username)
			msg := fmt.Sprintf(`permission with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		ok, err := p.es.deletePermission(req.Context(), username)
		if ok && err == nil {
			// Only update local state when proxy API has not been called
			// If proxy API would get called then it would automatically update the
			// state for all machines
			// Updating the local state again can cause insconsistency issues
			if util.ShouldProxyToACCAPI() {
				// Invoke ACCAPI
				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodDelete,
					URL:    "/_permission/" + username,
					Body:   nil,
				})
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				// Failed to update all nodes, return error response
				if res != nil {
					log.Errorln(logTag, ":", "error encountered deleting permission")
					bodyBytes, err := ioutil.ReadAll(res.Body)
					if err != nil {
						log.Errorln(logTag, ":", err)
						util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
						return
					}
					util.WriteBackRaw(w, bodyBytes, res.StatusCode)
					return
				}
			} else {
				// clear user details locally
				auth.ClearLocalUser(username)
			}
			msg := fmt.Sprintf(`permission with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`permission with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
		return
	}
}

func (p *permissions) getPermissions() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		indices, err := index.FromContext(ctx)
		if err != nil {
			msg := "an error occurred while fetching permissions"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		reqUser, err := user.FromContext(req.Context())
		if reqUser == nil || err != nil {
			msg := fmt.Sprintf(`an error occurred while fetching the user details`)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		// if user is not an admin then throw unauthorized error
		if !*reqUser.IsAdmin && !reqUser.HasAction(user.AccessControl) {
			msg := fmt.Sprintf(`You are not authorized to access the permissions. Please contact your admin.`)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}
		raw, err := p.es.getPermissions(ctx, indices)
		if err != nil {
			msg := fmt.Sprintf(`an error occurred while fetching permissions`)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (p *permissions) getUserPermissions() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		owner, _, _ := req.BasicAuth()

		raw, err := p.es.getRawOwnerPermissions(req.Context(), owner)
		if err != nil {
			msg := fmt.Sprintf(`an error occurred while fetching permissions for "owner"="%s"`, owner)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (p *permissions) role() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		role := vars["name"]

		var raw []byte
		var perm permission.Permission
		if req.Method != http.MethodPost {
			var err error
			raw, err = p.es.getRawRolePermission(req.Context(), role)
			if raw == nil || err != nil {
				msg := fmt.Sprintf(`an error occurred while fetching permissions for role=%s`, role)
				log.Errorln(logTag, ":", msg, ":", err)
				util.WriteBackError(w, msg, http.StatusNotFound)
				return
			}
			err = json.Unmarshal(raw, &perm)
			if err != nil {
				msg := fmt.Sprintf(`an error occurred while fetching permissions for role=%s`, role)
				log.Errorln(logTag, ":", msg, ":", err)
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
			http.Redirect(w, req, "/_permission/"+perm.Username, http.StatusPermanentRedirect)
			return
		case http.MethodDelete:
			http.Redirect(w, req, "/_permission/"+perm.Username, http.StatusPermanentRedirect)
			return
		}
	}
}
