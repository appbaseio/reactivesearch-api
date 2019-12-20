package auth

import (
	"context"
	"crypto/rsa"
	"io/ioutil"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/plugins"
	"github.com/appbaseio/arc/util"
	"github.com/dgrijalva/jwt-go"
)

const (
	logTag                    = "[auth]"
	envUsersEsIndex           = "USERS_ES_INDEX"
	defaultUsersEsIndex       = ".users"
	envEsURL                  = "ES_CLUSTER_URL"
	envPermissionsEsIndex     = "PERMISSIONS_ES_INDEX"
	defaultPermissionsEsIndex = ".permissions"
	envPublicKeyEsIndex       = "PUBLIC_KEY_ES_INDEX"
	defaultPublicKeyEsIndex   = ".publickey"
	envJwtRsaPublicKeyLoc     = "JWT_RSA_PUBLIC_KEY_LOC"
	envJwtRoleKey             = "JWT_ROLE_KEY"
	settings                  = `{ "settings" : { "number_of_shards" : %d, "number_of_replicas" : %d } }`
	publicKeyDocID            = "_public_key"
)

var (
	singleton *Auth
	once      sync.Once
)

// Auth (TODO - clear cache after fixed entries: LRU?)
type Auth struct {
	mu              sync.Mutex
	credentialCache map[string]credential.AuthCredential
	jwtRsaPublicKey *rsa.PublicKey
	jwtRoleKey      string
	es              authService
}

// Instance returns the singleton instance of the auth plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *Auth {
	once.Do(func() {
		singleton = &Auth{
			credentialCache: make(map[string]credential.AuthCredential),
		}
	})
	return singleton
}

// Name returns the name of the plugin: [auth]
func (a *Auth) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (a *Auth) InitFunc() error {
	// fetch vars from env
	userIndex := os.Getenv(envUsersEsIndex)
	if userIndex == "" {
		userIndex = defaultUsersEsIndex
	}
	permissionIndex := os.Getenv(envPermissionsEsIndex)
	if permissionIndex == "" {
		permissionIndex = defaultPermissionsEsIndex
	}
	publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
	if publicKeyIndex == "" {
		publicKeyIndex = defaultPublicKeyEsIndex
	}
	var err error

	// initialize the dao
	a.es, err = initPlugin(userIndex, permissionIndex)
	if err != nil {
		return err
	}

	// Create public key index
	_, err = a.es.createIndex(publicKeyIndex, settings)
	if err != nil {
		return err
	}

	// Populate public key from ES
	record, err := a.es.getPublicKey(context.Background())
	if err != nil {
		jwtRsaPublicKeyLoc := os.Getenv(envJwtRsaPublicKeyLoc)
		if jwtRsaPublicKeyLoc != "" {
			// Read file from location
			var publicKeyBuf []byte
			publicKeyBuf, err = ioutil.ReadFile(jwtRsaPublicKeyLoc)
			if err != nil {
				log.Error(logTag, ": unable to read the public key file from environment, ", err)
			}
			var record = publicKey{}
			record.PublicKey = string(publicKeyBuf)
			record.RoleKey = a.jwtRoleKey
			_, err = a.savePublicKey(context.Background(), publicKeyIndex, record)
			if err != nil {
				log.Error(logTag, ": unable to save public key record from environment, ", err)
			}
		}
	} else {
		publicKeyBuf, err := util.DecodeBase64Key(record.PublicKey)
		if err != nil {
			log.Error(logTag, ": error parsing public key record, ", err)
		}
		a.jwtRsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyBuf)
		if err != nil {
			log.Error(logTag, ": error parsing public key record, ", err)
		}
		a.jwtRoleKey = record.RoleKey
	}

	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (a *Auth) Routes() []plugins.Route {
	return a.routes()
}

// Default empty middleware array function
func (a *Auth) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
