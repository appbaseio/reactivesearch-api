package es

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/util"
)

func (es *ES) classifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		method := r.Method
		reqACL, reqOp := es.categorize(method, path)

		params := r.URL.Query()
		stream := params.Get("stream")
		if stream == "true" {
			reqACL = acl.Streams
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, acl.CtxKey, &reqACL)
		ctx = context.WithValue(ctx, op.CtxKey, &reqOp)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func (es *ES) categorize(method, path string) (acl.ACL, op.Operation) {
	for _, api := range es.specs {
		for _, pattern := range api.regexps {
			// TODO: additional check for keywords?
			ok, err := regexp.MatchString(pattern, path)
			if err != nil {
				log.Printf("%s: malformed regexp %s: %v", logTag, pattern, err)
				continue
			}
			if ok && util.Contains(api.spec.Methods, method) && matchKeywords(api, path) {
				return api.acl, getOp(api.spec.Methods, method)
			}
		}
	}
	// TODO: should we classify it as misc and then return the result.
	log.Printf("%s: unable to find the category for path [%s]: %s, categorising as 'misc'",
		logTag, method, path)
	return acl.Misc, op.Read
}

func matchKeywords(api api, path string) bool {
	var count int
	tokens := strings.Split(path, "/")
	for _, token := range tokens {
		if strings.HasPrefix(token, "_") {
			if _, ok := api.keywords[token]; ok {
				return true
			}
			count++
		}
	}
	return count == 0
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
		operation = op.Read // TODO: correct default or panic?
	}
	return operation
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctxACL := r.Context().Value(acl.CtxKey)
		if ctxACL == nil {
			log.Printf("%s: cannot fetch *acl.ACL from request context\n", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		a := ctxACL.(*acl.ACL)

		ctxPermission := r.Context().Value(permission.CtxKey)
		if ctxPermission == nil {
			log.Printf("%s: cannot fetch *permission.Permission from request context\n", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		p := ctxPermission.(*permission.Permission)

		if !acl.Contains(p.ACLs, *a) {
			msg := fmt.Sprintf("permission with username=%s does not have '%s' acl",
				p.UserName, *a)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctxOp := r.Context().Value(op.CtxKey)
		if ctxOp == nil {
			log.Printf("%s: cannot fetch *op.Operation from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		operation := ctxOp.(*op.Operation)

		ctxPermission := r.Context().Value(permission.CtxKey)
		if ctxPermission == nil {
			log.Printf("%s: cannot fetch *permission.Permission from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		p := ctxPermission.(*permission.Permission)

		if !op.Contains(p.Ops, *operation) {
			msg := fmt.Sprintf("permission with username=%s does not have '%s' operation",
				p.UserName, operation)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}
