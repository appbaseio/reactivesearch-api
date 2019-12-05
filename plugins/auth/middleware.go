package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/middleware/validate"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/model/index"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/util"
	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
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
		classify.Op(),
		BasicAuth(),
		validate.Operation(),
		validate.Category(),
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
		if publicKeyIndex == "" {
			publicKeyIndex = defaultPublicKeyEsIndex
		}
		ctx := index.NewContext(req.Context(), []string{publicKeyIndex})
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		permissionCategory := category.Auth

		ctx := category.NewContext(req.Context(), &permissionCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

// BasicAuth middleware authenticates each requests against the basic auth credentials.
func BasicAuth() middleware.Middleware {
	return Instance().basicAuth
}

func (a *Auth) basicAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Printf("%s: *category.Category not found in request context: %v", logTag, err)
			util.WriteBackError(w, "error occurred while authenticating the request", http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: *op.Op not found the request context: %v", logTag, err)
			util.WriteBackError(w, "error occurred while authenticating the request", http.StatusInternalServerError)
			return
		}

		username, password, hasBasicAuth := req.BasicAuth()
		jwtToken, err := request.ParseFromRequest(req, request.AuthorizationHeaderExtractor, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			if a.jwtRsaPublicKey == nil {
				return nil, fmt.Errorf("No Public Key Registered")
			}
			return a.jwtRsaPublicKey, nil
		})
		if !hasBasicAuth && err != nil {
			var msg string
			if err == request.ErrNoTokenInRequest {
				msg = "Basic Auth or JWT is required"
			} else {
				msg = fmt.Sprintf("Unable to parse JWT: %v", err)
			}
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		role := ""
		if !hasBasicAuth {
			if claims, ok := jwtToken.Claims.(jwt.MapClaims); ok && jwtToken.Valid {
				if a.jwtRoleKey != "" && claims[a.jwtRoleKey] != nil {
					role = claims[a.jwtRoleKey].(string)
				} else if u, ok := claims["role"]; ok {
					role = u.(string)
				} else {
					util.WriteBackError(w, fmt.Sprintf("Invalid JWT"), http.StatusUnauthorized)
					return
				}
			} else {
				util.WriteBackError(w, fmt.Sprintf("Invalid JWT"), http.StatusUnauthorized)
				return
			}
		}
		// we don't know if the credentials provided here are of a 'user' or a 'permission'
		var obj credential.AuthCredential
		if role != "" {
			obj, err = a.es.getRolePermission(ctx, role)
			if err != nil || obj == nil {
				msg := fmt.Sprintf("No API credentials match with provided role: %s", role)
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		} else {
			obj, err = a.getCredential(ctx, username)
			if err != nil || obj == nil {
				msg := fmt.Sprintf("No API credentials match with provided username: %s", username)
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		var authenticated bool

		// since we are able to fetch a result with the given credentials, we
		// do not need to validate the username and password.
		switch obj.(type) {
		case *user.User:
			{
				// if the request is made to elasticsearch using user credentials, then the user has to be an admin
				reqUser := obj.(*user.User)
				if hasBasicAuth && bcrypt.CompareHashAndPassword([]byte(reqUser.Password), []byte(password)) != nil {
					util.WriteBackError(w, "invalid password", http.StatusUnauthorized)
					return
				}
				if reqCategory.IsFromES() {
					authenticated = *reqUser.IsAdmin
				} else {
					authenticated = true
				}

				// cache the user
				if _, ok := a.cachedCredential(username); !ok {
					a.cacheCredential(username, reqUser)
				}

				// store request user and credential identifier in the context
				ctx = credential.NewContext(ctx, credential.User)
				ctx = user.NewContext(ctx, reqUser)
				req = req.WithContext(ctx)
			}
		case *permission.Permission:
			{
				reqPermission := obj.(*permission.Permission)
				if hasBasicAuth && reqPermission.Password != password {
					util.WriteBackError(w, "invalid password", http.StatusUnauthorized)
					return
				}

				if reqCategory.IsFromES() {
					authenticated = true
				}

				// cache the permission
				if _, ok := a.cachedCredential(username); !ok {
					a.cacheCredential(username, reqPermission)
				}

				// store the request permission and credential identifier in the context
				ctx = credential.NewContext(ctx, credential.Permission)
				ctx = permission.NewContext(ctx, reqPermission)
				req = req.WithContext(ctx)
			}
		default:
			log.Printf("%s: unreachable state ...", logTag)
		}

		if !authenticated {
			util.WriteBackError(w, "invalid credentials provided", http.StatusUnauthorized)
			return
		}

		// remove user/permission from cache on write operation
		if *reqOp == op.Write || *reqOp == op.Delete {
			username := mux.Vars(req)["username"]
			a.removeCredentialFromCache(username)
		}

		h(w, req)
	}
}

func (a *Auth) getCredential(ctx context.Context, username string) (credential.AuthCredential, error) {
	c, ok := a.cachedCredential(username)
	if ok {
		return c, nil
	}
	return a.es.getCredential(ctx, username)
}

func (a *Auth) cachedCredential(username string) (credential.AuthCredential, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.credentialCache[username]; ok {
		return c, ok
	}
	return nil, false
}

func (a *Auth) removeCredentialFromCache(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.credentialCache, username)
}

func (a *Auth) cacheCredential(username string, c credential.AuthCredential) {
	if c == nil {
		log.Printf("%s: cannot cache 'nil' credential, skipping...", logTag)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.credentialCache[username] = c
}
