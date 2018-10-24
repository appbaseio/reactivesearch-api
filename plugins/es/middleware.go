package es

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/credential"
	"github.com/appbaseio-confidential/arc/internal/types/index"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
)

func (es *es) classifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		method := r.Method
		reqACL, reqOp, indices := es.categorize(method, path)

		params := r.URL.Query()
		stream := params.Get("stream")
		if stream == "true" {
			reqACL = acl.Streams
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, acl.CtxKey, &reqACL)
		ctx = context.WithValue(ctx, op.CtxKey, &reqOp)
		ctx = context.WithValue(ctx, index.CtxKey, indices)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func (es *es) categorize(method, path string) (acl.ACL, op.Operation, []string) {
	for _, api := range es.specs {
		for endpoint, pattern := range api.pathRegexps {
			// TODO: additional check for keywords?
			ok, err := regexp.MatchString(pattern, path)
			if err != nil {
				log.Printf("%s: malformed regexp %s: %v", logTag, pattern, err)
				continue
			}
			if ok && util.Contains(api.spec.Methods, method) && matchKeywords(api, path) {
				return api.acl, getOp(api.spec.Methods, method), getIndexName(endpoint, path)
			}
		}
	}
	// TODO: should we classify it as misc and then return the result.
	log.Printf("%s: unable to find the category for path [%s]: %s, categorising as 'misc'",
		logTag, method, path)
	return acl.Misc, op.Read, []string{}
}

func getIndexName(endpoint, requestPath string) []string {
	const indexVar = "{index}"
	if !strings.Contains(endpoint, indexVar) {
		return []string{}
	}

	endpointTokens := strings.Split(endpoint, "/")
	requestPathTokens := strings.Split(requestPath, "/")
	if len(endpointTokens) != len(requestPathTokens) {
		log.Printf("%s: invalid clissifier match for path=%s and pattern=%s",
			logTag, requestPath, endpoint)
		return []string{}
	}

	for i := 0; i < len(requestPath); i++ {
		if endpointTokens[i] == indexVar {
			names := strings.Split(requestPathTokens[i], ",")
			var indices []string
			for _, name := range names {
				indices = append(indices, strings.TrimSpace(name))
			}
			return indices
		}
	}

	return []string{}
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
		ctx := r.Context()

		errMsg := "An error occurred while validating request acl"
		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(r.Context())
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}
		if reqCredential == credential.User {
			reqUser, err := user.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !reqUser.HasACL(*reqACL) {
				msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" acl`, reqUser.Username, *reqACL)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		} else if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			if !reqPermission.HasACL(*reqACL) {
				msg := fmt.Sprintf(`Permission with "username"="%s" does not have "%s" acl`, reqPermission.Username, *reqACL)
				util.WriteBackMessage(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request op"
		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}
		if reqCredential == credential.User {
			reqUser, err := user.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !reqUser.CanDo(*reqOp) {
				msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" op`, reqUser.Username, reqOp)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		} else if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			if !reqPermission.CanDo(*reqOp) {
				msg := fmt.Sprintf(`Permission with "username"="%s" does not have "%s" operation`,
					reqPermission.Username, reqOp)
				util.WriteBackMessage(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, r)
	}
}

func validateIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request indices"
		ctxIndices := ctx.Value(index.CtxKey)
		if ctxIndices == nil {
			log.Printf("%s: unable to fetch indices from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		indices, ok := ctxIndices.([]string)
		if !ok {
			log.Printf("%s: unable to cast ctxIndices to []string\n", logTag)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if len(indices) == 0 {
			// cluster level route
			reqCredential, err := credential.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if reqCredential == credential.User {
				reqUser, err := user.FromContext(ctx)
				if err != nil {
					log.Printf("%s: %v\n", logTag, err)
					util.WriteBackError(w, errMsg, http.StatusInternalServerError)
					return
				}
				canAccess, err := reqUser.CanAccessIndex("*")
				if err != nil {
					log.Printf("%s: %v", logTag, err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if !canAccess {
					msg := fmt.Sprintf(`User with "username"="%s" is unauthorized to access cluster level routes`,
						reqUser.Username)
					util.WriteBackError(w, msg, http.StatusUnauthorized)
					return
				}
			} else if reqCredential == credential.Permission {
				reqPermission, err := permission.FromContext(ctx)
				if err != nil {
					log.Printf("%s: %v\n", logTag, err)
					util.WriteBackError(w, errMsg, http.StatusInternalServerError)
					return
				}
				canAccess, err := reqPermission.CanAccessIndex("*")
				if err != nil {
					log.Printf("%s: %v", logTag, err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if !canAccess {
					msg := fmt.Sprintf(`Permission with "username"="%s" is unauthorized to access cluster level routes`,
						reqPermission.Username)
					util.WriteBackError(w, msg, http.StatusUnauthorized)
					return
				}
			}
		} else {
			// index level route
			reqCredential, err := credential.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if reqCredential == credential.User {
				reqUser, err := user.FromContext(ctx)
				if err != nil {
					log.Printf("%s: %v\n", logTag, err)
					util.WriteBackError(w, errMsg, http.StatusInternalServerError)
					return
				}
				for _, index := range indices {
					canAccess, err := reqUser.CanAccessIndex(index)
					if err != nil {
						log.Printf("%s: %v\n", logTag, err)
						util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
						return
					}
					if !canAccess {
						msg := fmt.Sprintf(`User with "username"="%s" is unauthprized to access index names "%s"`,
							reqUser.Username, index)
						util.WriteBackError(w, msg, http.StatusUnauthorized)
						return
					}
				}
			} else if reqCredential == credential.Permission {
				reqPermission, err := permission.FromContext(ctx)
				if err != nil {
					log.Printf("%s: %v\n", logTag, err)
					util.WriteBackError(w, errMsg, http.StatusInternalServerError)
					return
				}
				for _, index := range indices {
					canAccess, err := reqPermission.CanAccessIndex(index)
					if err != nil {
						log.Printf("%s: %v\n", logTag, err)
						util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
						return
					}
					if !canAccess {
						msg := fmt.Sprintf(`Permission with "username"="%s" is unauthorized to access index named "%s"`,
							reqPermission.Username, index)
						util.WriteBackError(w, msg, http.StatusUnauthorized)
						return
					}
				}
			}
		}

		h(w, r)
	}
}
