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

		// TODO: temporary entry point to create admin user
		masterUserId := os.Getenv("USER_ID")
		masterPassword := os.Getenv("PASSWORD")
		if userId == masterUserId && password == masterPassword {
			h(w, r)
			return
		}

		acl := r.Context().Value(acl.CtxKey)
		if acl == nil { // not an es request
			u, err := a.es.getUser(userId)
			if err != nil {
				msg := fmt.Sprintf("Unable to fetch user with userId=%s", userId)
				log.Printf("%s: %s: %v", logTag, msg, err)
				util.WriteBackMessage(w, msg, http.StatusInternalServerError)
				return
			}
			if password != u.Password {
				util.WriteBackMessage(w, "Incorrect credentials", http.StatusUnauthorized)
				return
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, user.CtxKey, u)
			r = r.WithContext(ctx)
		} else {
			p, err := a.es.getPermission(userId)
			if err != nil {
				msg := fmt.Sprintf("Unable to fetch permission with username=%s", userId)
				log.Printf("%s: %s: %v", logTag, msg, err)
				util.WriteBackMessage(w, msg, http.StatusInternalServerError)
				return
			}
			if password != p.Password {
				util.WriteBackMessage(w, "Incorrect credentials", http.StatusUnauthorized)
				return
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, permission.CtxKey, p)
			r = r.WithContext(ctx)
		}

		h(w, r)
	}
}
