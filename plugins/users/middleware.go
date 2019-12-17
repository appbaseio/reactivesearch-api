package users

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/middleware/validate"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/index"
	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/plugins/auth"
	"github.com/appbaseio/arc/plugins/logs"
	"github.com/appbaseio/arc/util"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classifyIndices,
		logs.Recorder(),
		classify.Op(),
		auth.BasicAuth(),
		validate.Operation(),
		validate.Category(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		userCategory := category.User

		ctx := category.NewContext(req.Context(), &userCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := index.NewContext(req.Context(), []string{defaultUsersEsIndex})
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func isAdmin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "an error occurred while validating user admin", http.StatusInternalServerError)
			return
		}

		if !*reqUser.IsAdmin {
			msg := fmt.Sprintf(`user with "username"="%s" is not an admin`, reqUser.Username)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}
