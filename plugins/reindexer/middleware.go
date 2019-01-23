package reindexer

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/middleware/classify"
	"github.com/appbaseio-confidential/arc/middleware/validate"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/index"
	"github.com/appbaseio-confidential/arc/model/op"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/plugins/auth"
	"github.com/appbaseio-confidential/arc/util"
)

type chain struct {
	order.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classify.Op(),
		classify.Indices(),
		auth.BasicAuth(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCategory := category.User
		ctx := context.WithValue(r.Context(), category.CtxKey, &requestCategory)
		r = r.WithContext(ctx)
		h(w, r)
	}
}

func identifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		indices := util.IndicesFromRequest(r)

		fmt.Println(indices)

		ctx := r.Context()
		ctx = context.WithValue(ctx, index.CtxKey, indices)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func validateCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "error occurred while validating request category"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !reqUser.HasCategory(category.User) {
			msg := fmt.Sprintf(`user with "username"="%s" does not have "%s" category`,
				reqUser.Username, category.Analytics)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "an error occurred while validating request op"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !reqUser.CanDo(*reqOp) {
			msg := fmt.Sprintf(`user with "username"="%s" does not have "%s" op`, reqUser.Username, *reqOp)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request indices"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		ctxIndices := ctx.Value(index.CtxKey)
		if ctxIndices == nil {
			log.Printf("%s: unable to fetch indices from request context\n", logTag)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}
		indices, ok := ctxIndices.([]string)
		if !ok {
			log.Printf("%s: unable to cast context indices to []string\n", logTag)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if len(indices) == 0 {
			// cluster level route
			ok, err := reqUser.CanAccessIndex("*")
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, `Invalid index pattern "*"`, http.StatusUnauthorized)
				return
			}
			if !ok {
				util.WriteBackError(w, "User is unauthorized to access cluster level routes", http.StatusUnauthorized)
				return
			}
		} else {
			// index level route
			for _, indexName := range indices {
				ok, err := reqUser.CanAccessIndex(indexName)
				if err != nil {
					msg := fmt.Sprintf(`Invalid index pattern encountered "%s"`, indexName)
					log.Printf("%s: invalid index pattern encountered %s: %v\n", logTag, indexName, err)
					util.WriteBackError(w, msg, http.StatusUnauthorized)
					return
				}

				if !ok {
					msg := fmt.Sprintf(`User is unauthorized to access index names "%s"`, indexName)
					util.WriteBackError(w, msg, http.StatusUnauthorized)
					return
				}
			}
		}

		h(w, r)
	}
}
