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
)

func (a *Auth) BasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, password, ok := r.BasicAuth()
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
			var p *permission.Permission

			// check in the cache
			p, ok := a.cachedPermission(userId)
			if !ok {
				p, err = a.es.getPermission(userId)
				if err != nil {
					msg := fmt.Sprintf(`Unable to fetch permission with "creator"="%s"`, userId)
					log.Printf("%s: %s: %v", logTag, msg, err)
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				// store in the cache
				a.cachePermission(userId, p)
			}

			if password != p.Password {
				util.WriteBackMessage(w, "Incorrect credentials", http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, permission.CtxKey, p)
			r = r.WithContext(ctx)
		} else {
			var err error
			var u *user.User

			// if we are patching a user or a permission, we must clear their
			// respective objects from the cache, otherwise the changes won't be
			// reflected the next time user tries to get the user or permission object.
			if r.Method == http.MethodPatch {
				switch *reqACL {
				case acl.User:
					a.removeUserFromCache(userId)
				case acl.Permission:
					a.removePermissionFromCache(userId)
				}
			}

			u, err = a.isMaster()
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if u != nil {
				ctx := r.Context()
				ctx = context.WithValue(ctx, user.CtxKey, u)
				r = r.WithContext(ctx)
				h(w, r)
				return
			}

			// check in the cache
			u, ok := a.cachedUser(userId)
			if !ok {
				u, err = a.es.getUser(userId)
				if err != nil {
					msg := fmt.Sprintf(`Unable to fetch user with "user_id"="%s"`, userId)
					log.Printf("%s: %s: %v", logTag, msg, err)
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				// store in the cache
				a.cacheUser(userId, u)
			}

			if password != u.Password {
				util.WriteBackMessage(w, "Incorrect credentials", http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, user.CtxKey, u)
			r = r.WithContext(ctx)
		}

		h(w, r)
	}
}

func (a *Auth) cachedUser(userId string) (*user.User, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.usersCache[userId]
	return u, ok
}

func (a *Auth) cacheUser(userId string, u *user.User) {
	if u == nil {
		log.Printf("%s: cannot cache 'nil' user, skipping...", logTag)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.usersCache[userId] = u
}

func (a *Auth) removeUserFromCache(userId string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.usersCache, userId)
}

func (a *Auth) createAdminUser(userId, password string) (*user.User, error) {
	u := user.NewAdmin(userId, password)
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
	log.Printf("%s: username=%s, password=%s", logTag, p.UserName, p.Password)
	return p, nil
}

func (a *Auth) isMaster() (*user.User, error) {
	masterUser, masterPassword := os.Getenv("USER_ID"), os.Getenv("PASSWORD")
	if masterUser == "" && masterPassword == "" {
		return nil, nil
	}

	master, err := a.es.getUser(masterUser)
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
