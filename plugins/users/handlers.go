package users

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/util"
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
			// Subscribe to down time alerts
			if newUser.HasAction(user.DowntimeAlerts) {
				err := subscribeToDowntimeAlert(newUser.Email)
				if err != nil {
					log.Errorln(logTag, err.Error())
				}
			}
			util.WriteBackRaw(w, rawUser, http.StatusCreated)
			return
		}

		msg := fmt.Sprintf(`an error occurred while creating a user with "username"="%s": %v`, userBody.Username, err)
		log.Println(logTag, ":", msg)
		util.WriteBackError(w, msg, http.StatusInternalServerError)
		return
	}
}

func (u *Users) patchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		username, _, _ := req.BasicAuth()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// clear user details locally
			auth.ClearLocalUser(username)
			util.WriteBackMessage(w, "User is updated successfully", http.StatusOK)
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
			// Only update local state when proxy API has not been called
			// If proxy API would get called then it would automatically update the
			// state for all machines
			// Updating the local state again can cause insconsistency issues
			if util.ShouldProxyToACCAPI() {
				// Invoke ACCAPI
				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodPatch,
					URL:    "/_user",
					Body:   nil,
				})
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				// Failed to update all nodes, return error response
				if res != nil {
					log.Errorln(logTag, ":", "error encountered updating user")
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
			// Subscribe to down time alerts
			if patch["allowed_actions"] != nil {
				actions, ok := patch["allowed_actions"].([]user.UserAction)
				if ok {
					email, ok := patch["email"].(string)
					if ok {
						// subscribe to alerts if user has `downtime` action
						// else unsubscribe to alerts
						if HasAction(actions, user.DowntimeAlerts) {
							err := subscribeToDowntimeAlert(email)
							if err != nil {
								log.Errorln(logTag, err.Error())
							}
						} else {
							err := unsubscribeToDowntimeAlert(email)
							if err != nil {
								log.Errorln(logTag, err.Error())
							}
						}
					}
				}
			}

			util.WriteBackMessage(w, "User is updated successfully", http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
		return
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
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// clear user details locally
			auth.ClearLocalUser(username)
			util.WriteBackMessage(w, "User is updated successfully", http.StatusOK)
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
			// Only update local state when proxy API has not been called
			// If proxy API would get called then it would automatically update the
			// state for all machines
			// Updating the local state again can cause insconsistency issues
			if util.ShouldProxyToACCAPI() {
				// Invoke ACCAPI
				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodPatch,
					URL:    "/_user/" + username,
					Body:   nil,
				})
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				// Failed to update all nodes, return error response
				if res != nil {
					log.Errorln(logTag, ":", "error encountered updating user")
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
			// Subscribe to down time alerts
			if patch["allowed_actions"] != nil {
				actions, ok := patch["allowed_actions"].([]user.UserAction)
				if ok {
					email, ok := patch["email"].(string)
					if ok {
						// subscribe to alerts if user has `downtime` action
						// else unsubscribe to alerts
						if HasAction(actions, user.DowntimeAlerts) {
							err := subscribeToDowntimeAlert(email)
							if err != nil {
								log.Errorln(logTag, err.Error())
							}
						} else {
							err := unsubscribeToDowntimeAlert(email)
							if err != nil {
								log.Errorln(logTag, err.Error())
							}
						}
					}
				}
			}

			util.WriteBackMessage(w, "User is updated successfully", http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
		return
	}
}

func (u *Users) deleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		username, _, _ := req.BasicAuth()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// delete user details locally
			auth.ClearLocalUser(username)
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		userDetails, err := u.es.getUser(req.Context(), username)
		if err != nil {
			msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
			log.Errorln(logTag, ":", msg, ":", err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		ok, err := u.es.deleteUser(req.Context(), username)
		if ok && err == nil {
			// Only update local state when proxy API has not been called
			// If proxy API would get called then it would automatically update the
			// state for all machines
			// Updating the local state again can cause insconsistency issues
			if util.ShouldProxyToACCAPI() {
				// Invoke ACCAPI
				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodDelete,
					URL:    "/_user",
					Body:   nil,
				})
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				// Failed to update all nodes, return error response
				if res != nil {
					log.Errorln(logTag, ":", "error encountered deleting user")
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
				// delete user details locally
				auth.ClearLocalUser(username)
			}

			// Unsubscribe to downtime alerts
			if userDetails.HasAction(user.DowntimeAlerts) {
				err := unsubscribeToDowntimeAlert(userDetails.Email)
				if err != nil {
					log.Errorln(logTag, err.Error())
				}
			}
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}

		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
		return
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
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// delete user details locally
			auth.ClearLocalUser(username)
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}
		userDetails, err2 := u.es.getUser(req.Context(), username)
		if err2 != nil {
			msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
			log.Errorln(logTag, ":", msg, ":", err2)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		ok, err := u.es.deleteUser(req.Context(), username)
		if ok && err == nil {
			// Only update local state when proxy API has not been called
			// If proxy API would get called then it would automatically update the
			// state for all machines
			// Updating the local state again can cause insconsistency issues
			if util.ShouldProxyToACCAPI() {
				// Invoke ACCAPI
				res, err := util.ProxyACCAPI(util.ProxyConfig{
					Method: http.MethodDelete,
					URL:    "/_user/" + username,
					Body:   nil,
				})
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				// Failed to update all nodes, return error response
				if res != nil {
					log.Errorln(logTag, ":", "error encountered deleting user with username ", username)
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
				// delete user details locally
				auth.ClearLocalUser(username)
			}

			// Unsubscribe to downtime alerts
			if userDetails.HasAction(user.DowntimeAlerts) {
				err := unsubscribeToDowntimeAlert(userDetails.Email)
				if err != nil {
					log.Errorln(logTag, err.Error())
				}
			}
			msg := fmt.Sprintf(`user with "username"="%s" deleted`, username)
			util.WriteBackMessage(w, msg, http.StatusOK)
			return
		}
		msg := fmt.Sprintf(`user with "username"="%s" not found`, username)
		log.Errorln(logTag, ":", msg, ":", err)
		util.WriteBackError(w, msg, http.StatusNotFound)
		return
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
