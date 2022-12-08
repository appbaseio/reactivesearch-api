package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/domain"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
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
		telemetry.Recorder(),
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
		tenantInfo, err := domain.FromContext(req.Context())
		if err != nil {
			log.Errorln("error while reading domain from context")
			telemetry.WriteBackErrorWithTelemetry(req, w, "Please make sure that you're using a tenant Id. If the issue persists please contact support@appbase.io with your domain or registered e-mail address.", http.StatusBadRequest)
			return
		}
		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ": *category.Category not found in request context:", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error occurred while authenticating the request", http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ": *op.Op not found the request context:", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error occurred while authenticating the request", http.StatusInternalServerError)
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
			w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
			return
		}

		role := ""
		if !hasBasicAuth {
			if claims, ok := jwtToken.Claims.(jwt.MapClaims); ok && jwtToken.Valid {
				if a.jwtRoleKey[tenantInfo.Raw] != "" && claims[a.jwtRoleKey[tenantInfo.Raw]] != nil {
					role = claims[a.jwtRoleKey[tenantInfo.Raw]].(string)
				} else if u, ok := claims["role"]; ok {
					role = u.(string)
				} else {
					w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
					telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf("Invalid JWT"), http.StatusUnauthorized)
					return
				}
			} else {
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf("Invalid JWT"), http.StatusUnauthorized)
				return
			}
		}
		// we don't know if the credentials provided here are of a 'user' or a 'permission'
		var obj credential.AuthCredential
		if role != "" {
			obj, err = a.es.getRolePermission(ctx, role)
			if err != nil || obj == nil {
				msg := fmt.Sprintf("No API credentials match with provided role: %s", role)
				log.Errorln(logTag, ":", err)
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
				return
			}
		} else {
			obj, err = a.getCredential(ctx, username)
			if err != nil || obj == nil {
				msg := fmt.Sprintf("No API credentials match with provided username: %s", username)
				log.Warnln(logTag, ":", err)
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
				return
			}
		}

		var authenticated bool
		var errorMsg = "invalid credentials provided"

		// since we are able to fetch a result with the given credentials, we
		// do not need to validate the username and password.
		switch obj.(type) {
		case *user.User:
			{

				reqUser := obj.(*user.User)

				// track `user` middleware
				ctx := trackplugin.TrackPlugin(ctx, "au")
				req = req.WithContext(ctx)

				// No need to validate if already validated before
				if hasBasicAuth && !IsPasswordExist(tenantInfo.Raw, reqUser.Username, password) && bcrypt.CompareHashAndPassword([]byte(reqUser.Password), []byte(password)) != nil {
					w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
					telemetry.WriteBackErrorWithTelemetry(req, w, "invalid password", http.StatusUnauthorized)
					return
				}
				// Save validated username to avoid the bcrypt comparison
				SavePassword(tenantInfo.Raw, reqUser.Username, password)

				// ignore es auth for root route to fetch the cluster details
				if (req.Method == http.MethodGet || req.Method == http.MethodHead) && req.RequestURI == "/" {
					authenticated = true
				} else if *reqUser.IsAdmin {
					authenticated = true
				} else if reqCategory.IsFromES() {
					// if the request is made to elasticsearch using user credentials,
					// then allow the access based on the categories present
					if reqUser.HasCategory(*reqCategory) {
						authenticated = true
					} else {
						errorMsg = "user not allowed to access elasticsearch"
					}
				} else if reqCategory.IsFromRS() {
					// if the request is made to reactivesearch api using user credentials,
					// then allow the access based on the `reactivesearch` category
					if reqUser.HasCategory(category.ReactiveSearch) {
						authenticated = true
						errorMsg = "user not allowed to access reactivesearch API"
					} else {
						errorMsg = "user not allowed to access elasticsearch"
					}
				} else {
					authenticated = true
				}

				// cache the user
				if _, ok := GetCachedCredential(tenantInfo.Raw, username); !ok {
					SaveCredentialToCache(tenantInfo.Raw, username, reqUser)
				}

				// store request user and credential identifier in the context
				ctx = credential.NewContext(ctx, credential.User)
				ctx = user.NewContext(ctx, reqUser)
				req = req.WithContext(ctx)
			}
		case *permission.Permission:
			{
				// track `permission` middleware
				ctx := trackplugin.TrackPlugin(ctx, "ap")
				req = req.WithContext(ctx)

				reqPermission := obj.(*permission.Permission)
				if hasBasicAuth && reqPermission.Password != password {
					w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
					telemetry.WriteBackErrorWithTelemetry(req, w, "invalid password", http.StatusUnauthorized)
					return
				}
				// ignore es auth for root route to fetch the cluster details
				if req.Method == http.MethodGet && req.RequestURI == "/" {
					authenticated = true
				} else if reqPermission.HasCategory(*reqCategory) {
					authenticated = true
				} else {
					str := (*reqCategory).String()
					errorMsg = "credential is not allowed to access" + " " + str
				}

				// cache the permission
				if _, ok := GetCachedCredential(tenantInfo.Raw, username); !ok {
					SaveCredentialToCache(tenantInfo.Raw, username, reqPermission)
				}

				// store the request permission and credential identifier in the context
				ctx = credential.NewContext(ctx, credential.Permission)
				ctx = permission.NewContext(ctx, reqPermission)
				req = req.WithContext(ctx)
			}
		default:
			log.Println(logTag, ": unreachable state ...")
		}

		if !authenticated {
			w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
			telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusUnauthorized)
			return
		}

		// remove user/permission from cache on write operation
		if *reqOp == op.Write || *reqOp == op.Delete {
			username := mux.Vars(req)["username"]
			RemoveCredentialFromCache(tenantInfo.Raw, username)
		}

		h(w, req)
	}
}

func (a *Auth) getCredential(ctx context.Context, username string) (credential.AuthCredential, error) {
	tenantInfo, _ := domain.FromContext(ctx)
	c, ok := GetCachedCredential(tenantInfo.Raw, username)
	if ok {
		return c, nil
	}
	return a.es.getCredential(ctx, username)
}

// GetCachedCredential returns the cached credential
func GetCachedCredential(domain string, username string) (credential.AuthCredential, bool) {
	CredentialCache.mu.Lock()
	defer CredentialCache.mu.Unlock()
	if domainCache, ok := CredentialCache.cache[domain]; ok {
		if c, ok := domainCache[username]; ok {
			return c, ok
		}
	}
	return nil, false
}

// GetCachedCredentials returns the cached credentials
func GetCachedCredentials() map[string]map[string]credential.AuthCredential {
	CredentialCache.mu.Lock()
	defer CredentialCache.mu.Unlock()
	return CredentialCache.cache
}

// GetCachedCredentials returns the cached credentials for a domain
func GetCachedCredentialsByDomain(domain string) []credential.AuthCredential {
	CredentialCache.mu.Lock()
	defer CredentialCache.mu.Unlock()
	var credentials []credential.AuthCredential
	if domainCache, ok := CredentialCache.cache[domain]; ok {
		for _, v := range domainCache {
			credentials = append(credentials, v)
		}
	}
	return credentials
}

// RemoveCredentialFromCache removes the credential from the cache
func RemoveCredentialFromCache(domain string, username string) {
	CredentialCache.mu.Lock()
	defer CredentialCache.mu.Unlock()
	delete(CredentialCache.cache, username)
}

// SaveCredentialToCache saves the credential to the cache
func SaveCredentialToCache(domain string, username string, c credential.AuthCredential) {
	if c == nil {
		log.Println(logTag, ": cannot cache 'nil' credential, skipping...")
		return
	}
	CredentialCache.mu.Lock()
	if _, ok := CredentialCache.cache[domain]; ok {
		CredentialCache.cache[domain][username] = c
	} else {
		CredentialCache.cache[domain] = map[string]credential.AuthCredential{
			username: c,
		}
	}
	CredentialCache.mu.Unlock()
}
