package util

import (
	"net/http"
	"sync"
	"testing"
)

func resetESAuthForTest(t *testing.T) {
	t.Helper()
	esAuthOnce = sync.Once{}
	esAPIKey = ""
	esAuthErr = nil
}

func TestValidateESAuthConfig_APIKeyWithURLCredentials(t *testing.T) {
	resetESAuthForTest(t)
	t.Setenv("ES_CLUSTER_URL", "https://user:pass@cluster.example.com:443")
	t.Setenv("ES_API_KEY", "abc123")

	err := validateESAuthConfig()
	if err == nil {
		t.Fatal("expected error when ES_API_KEY is set with URL credentials")
	}
}

func TestValidateESAuthConfig_APIKeyOnly(t *testing.T) {
	resetESAuthForTest(t)
	t.Setenv("ES_CLUSTER_URL", "https://cluster.example.com:443")
	t.Setenv("ES_API_KEY", "abc123")

	if err := validateESAuthConfig(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !UsesESAPIKey() {
		t.Fatal("expected API key auth to be enabled")
	}
}

func TestGetESURL_APIKeyModeReturnsCleanURL(t *testing.T) {
	resetESAuthForTest(t)
	t.Setenv("ES_CLUSTER_URL", "https://cluster.example.com:443")
	t.Setenv("ES_API_KEY", "abc123")

	got := GetESURL()
	want := "https://cluster.example.com:443"
	if got != want {
		t.Fatalf("GetESURL() = %q, want %q", got, want)
	}
}

func TestGetESURL_BasicAuthModePreservesEscapedCredentials(t *testing.T) {
	resetESAuthForTest(t)
	t.Setenv("ES_CLUSTER_URL", "https://user:pass@cluster.example.com:443")
	t.Setenv("ES_API_KEY", "")

	got := GetESURL()
	want := "https://user:pass@cluster.example.com:443"
	if got != want {
		t.Fatalf("GetESURL() = %q, want %q", got, want)
	}
}

func TestApplyESAuth_SetsAuthorizationHeader(t *testing.T) {
	resetESAuthForTest(t)
	t.Setenv("ES_CLUSTER_URL", "https://cluster.example.com:443")
	t.Setenv("ES_API_KEY", "encoded-key")

	req, err := http.NewRequest(http.MethodGet, "https://cluster.example.com:443", nil)
	if err != nil {
		t.Fatal(err)
	}

	ApplyESAuth(req)
	if got := req.Header.Get("Authorization"); got != "ApiKey encoded-key" {
		t.Fatalf("Authorization header = %q, want %q", got, "ApiKey encoded-key")
	}
}
