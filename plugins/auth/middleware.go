package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/credential"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/gorilla/mux"
)

// BasicAuth middleware that authenticates each requests against the basic auth credentials.
func (a *Auth) BasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		username, password, ok := r.BasicAuth()
		if !ok {
			util.WriteBackError(w, "Not logged in", http.StatusUnauthorized)
			return
		}

		obj, err := a.es.getCredential(username, password)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if obj == nil {
			msg := fmt.Sprintf(`Credential with "username"="%s" Not Found`, username)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		var (
			reqCredential credential.Credential
			reqUser       *user.User
			reqPermission *permission.Permission
		)

		reqPermission, ok = obj.(*permission.Permission)
		if ok {
			reqCredential = credential.Permission
		} else {
			reqUser, ok = obj.(*user.User)
			if ok {
				reqCredential = credential.User
			} else {
				msg := fmt.Sprintf(`Credential with "username"="%s" Not Found`, username)
				log.Printf(`%s: cannot cast obj "%v" to either permission.Permission or user.User`, logTag, obj)
				util.WriteBackError(w, msg, http.StatusNotFound)
				return
			}
		}

		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			msg := "An error occurred while authenticating the request"
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		if reqACL.IsFromES() {
			if reqCredential == credential.User {
				if !(*reqUser.IsAdmin) {
					msg := fmt.Sprintf(`User with "username"="%s" is not an admin`, username)
					util.WriteBackError(w, msg, http.StatusUnauthorized)
					return
				}
				if password != reqUser.Password {
					util.WriteBackError(w, "Incorrect credentials", http.StatusUnauthorized)
					return
				}
				ctx := r.Context()
				ctx = context.WithValue(ctx, credential.CtxKey, reqCredential)
				ctx = context.WithValue(ctx, user.CtxKey, reqUser)
				r = r.WithContext(ctx)
			} else {
				if password != reqPermission.Password {
					util.WriteBackMessage(w, "Incorrect credentials", http.StatusUnauthorized)
					return
				}
				ctx := r.Context()
				ctx = context.WithValue(ctx, credential.CtxKey, reqCredential)
				ctx = context.WithValue(ctx, permission.CtxKey, reqPermission)
				r = r.WithContext(ctx)
			}
		} else {
			reqUser, err = a.isMaster(username, password)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackMessage(w, "Unable create a master user", http.StatusInternalServerError)
				return
			}
			if reqUser != nil {
				ctx := r.Context()
				ctx = context.WithValue(ctx, user.CtxKey, reqUser)
				r = r.WithContext(ctx)
				h(w, r)
				return
			}

			// if we are patching a user or a permission, we must clear their
			// respective objects from the cache, otherwise the changes won't be
			// reflected the next time user tries to get the user or permission object.
			if r.Method == http.MethodPatch || r.Method == http.MethodDelete {
				switch *reqACL {
				case acl.User:
					a.removeUserFromCache(username)
				case acl.Permission:
					username := mux.Vars(r)["username"]
					a.removePermissionFromCache(username)
				}
			}

			// check in the cache
			reqUser, ok = a.cachedUser(username)
			if !ok {
				reqUser, err = a.es.getUser(username)
				if err != nil {
					msg := fmt.Sprintf(`User with "user_id"="%s" Not Found`, username)
					log.Printf("%s: %s: %v", logTag, msg, err)
					util.WriteBackError(w, msg, http.StatusNotFound)
					return
				}
				// store in the cache
				a.cacheUser(username, reqUser)
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
	u, err := user.NewAdmin(userID, password)
	if err != nil {
		return nil, err
	}

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
	p, err := permission.NewAdmin(creator)
	if err != nil {
		return nil, err
	}

	ok, err := a.es.putPermission(*p)
	if !ok || err != nil {
		return nil, err
	}

	return p, nil
}

func (a *Auth) isMaster(username, password string) (*user.User, error) {
	masterUser, masterPassword := os.Getenv("USERNAME"), os.Getenv("PASSWORD")
	if masterUser != username && masterPassword != password {
		return nil, nil
	}

	master, err := a.es.getUser(username)
	if err != nil {
		log.Printf("%s: master user doesn't exists, creating one... : %v", logTag, err)
		master, err = user.NewAdmin(masterUser, masterPassword)
		if err != nil {
			return nil, err
		}
		ok, err := a.es.putUser(*master)
		if !ok || err != nil {
			return nil, fmt.Errorf("%s: unable to create master user: %v", logTag, err)
		}
	}

	return master, nil
}
