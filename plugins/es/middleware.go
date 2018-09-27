package es

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/util"
)

func (es *ES) classifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		method := r.Method
		c, o := es.categorize(method, path)

		params := r.URL.Query()
		stream := params.Get("stream")
		if stream == "true" {
			c = category.Streams
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, category.CtxKey, c)
		ctx = context.WithValue(ctx, op.CtxKey, o)
		h(w, r.WithContext(ctx))
	}
}

func (es *ES) categorize(method, path string) (category.Category, op.Operation) {
	for _, api := range es.specs {
		for _, pattern := range api.regexps {
			ok, err := regexp.MatchString(pattern, path)
			if err != nil {
				log.Printf("%s: malformed regexp %s: %v", logTag, pattern, err)
				continue
			}
			if ok && util.Contains(api.spec.Methods, method) {
				return api.category, getOp(api.spec.Methods, method)
			}
		}
	}
	// TODO: should we classify it as misc and then return the result.
	log.Printf("%s: unable to find the category for path [%s]: %s, categorising as 'misc'",
		logTag, method, path)
	return category.Misc, op.Read
}

func getOp(methods []string, method string) op.Operation {
	var operation op.Operation
	switch method {
	case http.MethodGet:
		operation = op.Read
	case http.MethodPost:
		if util.Contains(methods, http.MethodGet) {
			operation = op.Read
		} else {
			operation = op.Write
		}
	case http.MethodPut:
		operation = op.Write
	case http.MethodHead:
		operation = op.Read
	case http.MethodDelete:
		operation = op.Delete
	default:
		operation = op.Noop
	}
	return operation
}
