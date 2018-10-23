package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/gorilla/mux"
)

// BasicAuth middleware that authenticates each requests against the basic auth credentials.
func (a *Auth) BasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, password, ok := r.BasicAuth()
		if !ok {
			util.WriteBackMessage(w, "Not logged in", http.StatusUnauthorized)
			return
		}

		ctxACL := r.Context().Value(acl.CtxKey)
		if ctxACL == nil {
			log.Printf("%s: request must be classified before it is authenticated", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqACL, ok := ctxACL.(*acl.ACL)
		if !ok {
			log.Printf("%s: unable to cast context acl %v to type *acl.ACL", logTag, ctxACL)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if reqACL.IsFromES() {
			var err error
			var reqPermission *permission.Permission

			// check in the cache
			reqPermission, ok = a.cachedPermission(userID)
			if !ok {
				reqPermission, err = a.es.getPermission(userID)
				if err != nil {
					msg := fmt.Sprintf(`Unable to fetch permission with "username"="%s"`, userID)
					log.Printf("%s: %s: %v", logTag, msg, err)
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				// store in the cache
				a.cachePermission(userID, reqPermission)
			}

			if password != reqPermission.Password {
				util.WriteBackMessage(w, "Incorrect credentials", http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, permission.CtxKey, reqPermission)
			r = r.WithContext(ctx)
		} else {
			var err error
			var reqUser *user.User

			// if we are patching a user or a permission, we must clear their
			// respective objects from the cache, otherwise the changes won't be
			// reflected the next time user tries to get the user or permission object.
			if r.Method == http.MethodPatch || r.Method == http.MethodDelete {
				switch *reqACL {
				case acl.User:
					a.removeUserFromCache(userID)
				case acl.Permission:
					username := mux.Vars(r)["username"]
					a.removePermissionFromCache(username)
				}
			}

			// master user
			reqUser, err = a.isMaster(userID, password)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if reqUser != nil {
				ctx := r.Context()
				ctx = context.WithValue(ctx, user.CtxKey, reqUser)
				r = r.WithContext(ctx)
				h(w, r)
				return
			}

			// check in the cache
			reqUser, ok = a.cachedUser(userID)
			if !ok {
				reqUser, err = a.es.getUser(userID)
				if err != nil {
					msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, userID)
					log.Printf("%s: %s: %v", logTag, msg, err)
					util.WriteBackError(w, msg, http.StatusNotFound)
					return
				}
				// store in the cache
				a.cacheUser(userID, reqUser)
			}

			if password != reqUser.Password {
				util.WriteBackMessage(w, "Incorrect credentials", http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, user.CtxKey, reqUser)
			r = r.WithContext(ctx)
		}

		h(w, r)
	}
}

func (a *Auth) cachedUser(userID string) (*user.User, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.usersCache[userID]
	return u, ok
}

func (a *Auth) cacheUser(userID string, u *user.User) {
	if u == nil {
		log.Printf("%s: cannot cache 'nil' user, skipping...", logTag)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.usersCache[userID] = u
}

func (a *Auth) removeUserFromCache(userID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.usersCache, userID)
}

func (a *Auth) createAdminUser(userID, password string) (*user.User, error) {
	u := user.NewAdmin(userID, password)
	ok, err := a.es.putUser(*u)
	if !ok || err != nil {
		return nil, err
	}
	return u, nil
}

func (a *Auth) cachedPermission(username string) (*permission.Permission, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.permissionsCache[username]
	return p, ok
}

func (a *Auth) cachePermission(username string, p *permission.Permission) {
	if p == nil {
		log.Printf("%s: cannot cache 'nil' permission, skipping...", logTag)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.permissionsCache[username] = p
}

func (a *Auth) removePermissionFromCache(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.permissionsCache, username)
}

func (a *Auth) createAdminPermission(creator string) (*permission.Permission, error) {
	p := permission.NewAdmin(creator)
	ok, err := a.es.putPermission(*p)
	if !ok || err != nil {
		return nil, err
	}
	log.Printf("%s: username=%s, password=%s", logTag, p.Username, p.Password)
	return p, nil
}

func (a *Auth) isMaster(userID, password string) (*user.User, error) {
	masterUser, masterPassword := os.Getenv("USER_ID"), os.Getenv("PASSWORD")
	if masterUser != userID && masterPassword != password {
		return nil, nil
	}

	master, err := a.es.getUser(userID)
	if err != nil {
		log.Printf("%s: master user doesn't exists, creating one... : %v", logTag, err)
		master = user.NewAdmin(masterUser, masterPassword)
		ok, err := a.es.putUser(*master)
		if !ok || err != nil {
			return nil, fmt.Errorf("%s: unable to create master user: %v", logTag, err)
		}
	}

	return master, nil
}
