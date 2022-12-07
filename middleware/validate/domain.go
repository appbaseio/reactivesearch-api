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

func ValidateDomain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if util.MultiTenant {
			domainName := req.Header.Get("X_REACTIVESEARCH_DOMAIN")
			if util.IsDevelopmentEnv && strings.TrimSpace(domainName) == "" {
				domainName = testDomain
			}

			if strings.TrimSpace(domainName) == "" {
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, "domain name is required", http.StatusUnauthorized)
				return
			} else {
				// encrypt domain and update context
				key := os.Getenv("DOMAIN_NAME_ENCRYPTION_KEY")
				ciphertext, err := Encrypt([]byte(key), []byte(domainName))
				if err != nil {
					telemetry.WriteBackErrorWithTelemetry(req, w, "error encrypting domain name: "+err.Error(), http.StatusInternalServerError)
					return
				}
				encryptedDomain := fmt.Sprintf("%0x", ciphertext)
				ctx := domain.NewContext(req.Context(), domain.DomainInfo{
					Encrypted: string(encryptedDomain),
					Raw:       domainName,
				})
				next.ServeHTTP(w, req.WithContext(ctx))
				return
			}
		}
		next.ServeHTTP(w, req)
	})
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
