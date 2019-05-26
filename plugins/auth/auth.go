package auth

import (
	"crypto/rsa"
	"github.com/dgrijalva/jwt-go"
	"io/ioutil"
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/plugins"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	logTag                    = "[auth]"
	envEsURL                  = "ES_CLUSTER_URL"
	envUsersEsIndex           = "USERS_ES_INDEX"
	defaultUsersEsIndex       = ".users"
	envPermissionsEsIndex     = "PERMISSIONS_ES_INDEX"
	defaultPermissionsEsIndex = ".permissions"
	envJwtRsaPublicKeyLoc     = "JWT_RSA_PUBLIC_KEY_LOC"
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
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	userIndex := os.Getenv(envUsersEsIndex)
	if userIndex == "" {
		userIndex = defaultUsersEsIndex
	}
	permissionIndex := os.Getenv(envPermissionsEsIndex)
	if permissionIndex == "" {
		permissionIndex = defaultPermissionsEsIndex
	}
	var err error
	jwtRsaPublicKeyLoc := os.Getenv(envJwtRsaPublicKeyLoc)
	if jwtRsaPublicKeyLoc != "" {
		var publicKeyBuf []byte
		publicKeyBuf, err = ioutil.ReadFile(jwtRsaPublicKeyLoc)
		if err != nil {
			return err
		}
		a.jwtRsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyBuf)
		if err != nil {
			return err
		}
	}

	// initialize the dao
	a.es, err = newClient(esURL, userIndex, permissionIndex)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (a *Auth) Routes() []plugins.Route {
	return []plugins.Route{}
}
