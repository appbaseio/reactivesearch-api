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

// TODO: cache users for fixed interval?
func (a *Auth) BasicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, password, ok := r.BasicAuth()
		if !ok {
			util.WriteBackMessage(w, "Not logged in", http.StatusUnauthorized)
			return
		}

		masterUserId := os.Getenv("USER_ID")
		masterPassword := os.Getenv("PASSWORD")

		ctxACL := r.Context().Value(acl.CtxKey)
		if ctxACL == nil {
			log.Printf("%s: request must be classified before it is authenticated", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		obj, ok := ctxACL.(acl.ACL)
		if !ok {
			log.Printf("%s: unable to cast context acl %v to type acl.ACL", logTag, obj)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if obj.IsFromES() {
			var err error
			var p *permission.Permission

			// TODO: temporary entry point?
			if userId == masterUserId && password == masterPassword {
				p, err = a.createAdminPermission(userId)
				if err != nil {
					msg := fmt.Sprintf(`unable to create admin permission for "username"="%s"`,
						userId)
					log.Printf("%s: %s: %v", logTag, msg, err)
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				ctx := r.Context()
				ctx = context.WithValue(ctx, permission.CtxKey, p)
				r = r.WithContext(ctx)
				h(w, r)
				return
			}

			p, err = a.es.getPermission(userId)
			if err != nil {
				msg := fmt.Sprintf(`Unable to fetch permission with "username"="%s"`, userId)
				log.Printf("%s: %s: %v", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
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

			// TODO: temporary entry point?
			if userId == masterUserId && password == masterPassword {
				u, err = a.createAdminUser(userId, password)
				if err != nil {
					msg := fmt.Sprintf(`unable to create admin user for "user_id"="%s"`, userId)
					log.Printf("%s: %s: %v", logTag, msg, err)
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				ctx := r.Context()
				ctx = context.WithValue(ctx, user.CtxKey, u)
				r = r.WithContext(ctx)
				h(w, r)
				return
			}

			u, err = a.es.getUser(userId)
			if err != nil {
				msg := fmt.Sprintf(`Unable to fetch user with "user_id"="%s"`, userId)
				log.Printf("%s: %s: %v", logTag, msg, err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
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

func (a *Auth) createAdminPermission(creator string) (*permission.Permission, error) {
	p := permission.NewAdmin(creator)
	ok, err := a.es.putPermission(*p)
	if !ok || err != nil {
		return nil, err
	}
	return p, nil
}

func (a *Auth) createAdminUser(userId, password string) (*user.User, error) {
	u := user.NewAdmin(userId, password)
	ok, err := a.es.putUser(*u)
	if !ok || err != nil {
		return nil, err
	}
	return u, nil
}
