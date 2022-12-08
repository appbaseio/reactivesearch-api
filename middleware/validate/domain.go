package validate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
)

const testDomain = "reactivesearch.test.io"

// DomainWhitelistedPaths will return an array of paths that do
// not require domain validation
func DomainWhitelistedPaths() []string {
	return []string{
		"/arc/health",
	}
}

// IsDomainWhitelisted will return whether or not the
// passed domain is whitelisted
func IsDomainWhitelisted(path string) bool {
	// Check if routes are blacklisted
	for _, route := range DomainWhitelistedPaths() {
		if strings.HasPrefix(path, route) {
			return true
		}
	}

	return false
}

func ValidateDomain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// If domain is whitelisted, we don't need to do anything
		if IsDomainWhitelisted(req.RequestURI) {
			next.ServeHTTP(w, req)
			return
		}

		req, parseErr := ParseDomainWithValidation(req)
		if parseErr != nil {
			if parseErr.Code == http.StatusUnauthorized {
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
			}

			telemetry.WriteBackErrorWithTelemetry(req, w, parseErr.Err.Error(), parseErr.Code)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// ParseDomainWithValidation will parse the domain from the passed
// request and inject it into the context.
//
// This method is defined separately so that it can be triggered
// manually in case the need comes. As of now, this method will be used
// in `pipelines` to fetch the domain name in the pipeline catch-all
// matcher.
func ParseDomainWithValidation(req *http.Request) (*http.Request, *util.ErrorWithCode) {
	if util.MultiTenant {
		domainName := req.Header.Get("X_REACTIVESEARCH_DOMAIN")
		if util.IsDevelopmentEnv && strings.TrimSpace(domainName) == "" {
			domainName = testDomain
		}

		if strings.TrimSpace(domainName) == "" {
			return req, &util.ErrorWithCode{
				Code: http.StatusUnauthorized,
				Err:  fmt.Errorf("domain name is required"),
			}
		} else {
			// encrypt domain and update context
			key := os.Getenv("DOMAIN_NAME_ENCRYPTION_KEY")
			ciphertext, err := Encrypt([]byte(key), []byte(domainName))
			if err != nil {
				return req, &util.ErrorWithCode{
					Code: http.StatusInternalServerError,
					Err:  fmt.Errorf("error encrypting domain name: " + err.Error()),
				}
			}
			encryptedDomain := fmt.Sprintf("%0x", ciphertext)
			ctx := domain.NewContext(req.Context(), domain.DomainInfo{
				Encrypted: string(encryptedDomain),
				Raw:       domainName,
			})
			return req.WithContext(ctx), nil
		}
	}

	return req, nil
}

func Encrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}
