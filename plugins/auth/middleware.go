package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/op"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/util"
	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"github.com/gorilla/mux"
)

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

		username, password, ok := req.BasicAuth()
		jwtToken, err := request.ParseFromRequest(req, request.AuthorizationHeaderExtractor, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			return a.jwtRsaPublicKey, nil
		})
		if !ok && err != nil {
			util.WriteBackError(w, "request credentials or jwt token is required", http.StatusUnauthorized)
			return
		}

		var checkPassword bool
		if ok {
			checkPassword = true
		} else if err == nil {
			checkPassword = false
			if claims, ok := jwtToken.Claims.(jwt.MapClaims); ok && jwtToken.Valid {
				username = claims["username"].(string)
			}
		}
		// we don't know if the credentials provided here are of a 'user' or a 'permission'
		obj, err := a.getCredential(ctx, username, password, checkPassword)
		if err != nil {
			msg := fmt.Sprintf("unable to fetch credentials with username: %s", username)
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		if obj == nil {
			msg := fmt.Sprintf("credential with username=%s, password=%s not found", username, password)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		var authenticated bool

		// since we are able to fetch a result with the given credentials, we
		// do not need to validate the username and password.
		switch obj.(type) {
		case *user.User:
			{
				// if the request is made to elasticsearch using user credentials, then the user has to be an admin
				reqUser := obj.(*user.User)
				if reqCategory.IsFromES() {
					authenticated = *reqUser.IsAdmin
				} else {
					authenticated = true
				}

				// cache the user
				if _, ok := a.cachedUser(username, password, checkPassword); !ok {
					a.cacheUser(username, reqUser)
				}

				// store request user and credential identifier in the context
				ctx = credential.NewContext(ctx, credential.User)
				ctx = user.NewContext(ctx, reqUser)
				req = req.WithContext(ctx)
			}
		case *permission.Permission:
			{
				if reqCategory.IsFromES() {
					authenticated = true
				}

				// cache the permission
				reqPermission := obj.(*permission.Permission)
				if _, ok := a.cachedPermission(username, password, checkPassword); !ok {
					a.cachePermission(username, reqPermission)
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
			switch *reqCategory {
			case category.User:
				if username, ok := mux.Vars(req)["username"]; ok {
					a.removeUserFromCache(username)
				} else {
					a.removeUserFromCache(username)
				}
			case category.Permission:
				// in case of permission, username is to be taken from request route
				username := mux.Vars(req)["username"]
				a.removePermissionFromCache(username)
			}
		}

		h(w, req)
	}
}

func (a *Auth) getCredential(ctx context.Context, username, password string, checkPassword bool) (interface{}, error) {
	// look for the credential in the cache first, if not found then make an es request
	user, ok := a.cachedUser(username, password, checkPassword)
	if ok {
		return user, nil
	}

	permission, ok := a.cachedPermission(username, password, checkPassword)
	if ok {
		return permission, nil
	}

	return a.es.getCredential(ctx, username, password, checkPassword)
}

func (a *Auth) cachedUser(userID, password string, checkPassword bool) (*user.User, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u, ok := a.usersCache[userID]; ok && (!checkPassword || u.Password == password) {
		return u, ok
	}
	return nil, false
}

func (a *Auth) cacheUser(userID string, u *user.User) {
	if u == nil {
		log.Printf("%s: cannot cache 'nil' user, skipping...", logTag)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.usersCache[userID] = u
}

func (a *Auth) removeUserFromCache(userID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.usersCache, userID)
}

func (a *Auth) cachedPermission(username, password string, checkPassword bool) (*permission.Permission, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.permissionsCache[username]; ok && (!checkPassword || p.Password == password) {
		return p, ok
	}
	return nil, false
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
