package util

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

const envESAPIKey = "ES_API_KEY"

var (
	esAuthOnce sync.Once
	esAPIKey   string
	esAuthErr  error
)

func loadESAuthConfig() {
	esAPIKey = strings.TrimSpace(os.Getenv(envESAPIKey))
	if esAPIKey == "" {
		return
	}

	if hasURLCredentials(os.Getenv("ES_CLUSTER_URL")) {
		esAuthErr = fmt.Errorf("ES_API_KEY cannot be used when ES_CLUSTER_URL contains username and password credentials")
	}
}

// validateESAuthConfig validates elasticsearch auth configuration.
func validateESAuthConfig() error {
	esAuthOnce.Do(loadESAuthConfig)
	return esAuthErr
}

// UsesESAPIKey reports whether upstream ES auth uses an API key.
func UsesESAPIKey() bool {
	if err := validateESAuthConfig(); err != nil {
		log.Fatal("Error encountered: ", err)
	}
	return esAPIKey != ""
}

// ESAuthHeaders returns request headers for API key auth when configured.
func ESAuthHeaders() http.Header {
	headers := make(http.Header)
	if UsesESAPIKey() {
		headers.Set("Authorization", "ApiKey "+esAPIKey)
	}
	return headers
}

// ApplyESAuth sets upstream elasticsearch authorization on an HTTP request.
func ApplyESAuth(req *http.Request) {
	if UsesESAPIKey() {
		req.Header.Set("Authorization", "ApiKey "+esAPIKey)
	}
}

func hasURLCredentials(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return strings.Contains(rawURL, "@")
	}
	return parsedURL.User != nil
}

func stripURLCredentials(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	parsedURL.User = nil
	return parsedURL.String(), nil
}
