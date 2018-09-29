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
		c, o := es.categorize(method, path)

		params := r.URL.Query()
		stream := params.Get("stream")
		if stream == "true" {
			c = acl.Streams
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, acl.CtxKey, c)
		ctx = context.WithValue(ctx, op.CtxKey, o)
		h(w, r.WithContext(ctx))
	}
}

func (es *ES) categorize(method, path string) (acl.ACL, op.Operation) {
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
	return acl.Misc, op.Read
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
		// TODO: handle?
	}
	return operation
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: add a check if set in classifier
		esACL := r.Context().Value(acl.CtxKey).(acl.ACL)
		p := r.Context().Value(permission.CtxKey).(*permission.Permission)
		if p == nil {
			// TODO: auth didn't fetch permission?
			log.Printf("%s: cannot fetch permission object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !acl.Contains(p.ACLs, esACL) {
			msg := fmt.Sprintf("permission with username=%s does not have '%s' acl",
				p.UserName, esACL)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		log.Printf("%s: validate acl: validated\n", logTag)
		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: add a check if set in classifier
		operation := r.Context().Value(op.CtxKey).(op.Operation)
		p := r.Context().Value(permission.CtxKey).(*permission.Permission)
		if p == nil {
			// TODO: auth didn't fetch permission?
			log.Printf("%s: cannot fetch permission object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !op.Contains(p.Ops, operation) {
			msg := fmt.Sprintf("permission with username=%s does not have '%s' operation",
				p.UserName, operation)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		log.Printf("%s: validate op: validated\n", logTag)
		h(w, r)
	}
}
