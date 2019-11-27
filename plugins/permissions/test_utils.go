package permissions

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/appbaseio/arc/errors"
)

func compareErrs(expectedErr string, actual error) bool {
	if actual == nil {
		if expectedErr == "" {
			return true
		}
		return false
	}

	return expectedErr == actual.Error()
}

func (p *permissions) mockInitFunc() error {
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}

	index := os.Getenv(envPermissionEsIndex)
	if index == "" {
		index = defaultPermissionsEsIndex
	}

	client, err := newStubClient(url, index, "mapping")
	if err != nil {
		return err
	}

	p.es = client
	return nil
}

func newStubClient(url, indexName, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{
		url:       url,
		indexName: indexName,
		mapping:   mapping,
	}
	return es, nil
}

type ServerSetup struct {
	Method, Path, Body, Response string
	HTTPStatus                   int
}

// This function is a modified version of: https://github.com/github/vulcanizer/blob/master/es_test.go
func buildTestServer(t *testing.T, setups []*ServerSetup) *httptest.Server {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, _ := ioutil.ReadAll(r.Body)
		requestBody := string(requestBytes)

		var s string
		matched := false
		for _, setup := range setups {
			s = setup.Method + ":" + setup.Path
			if r.Method == setup.Method && r.URL.EscapedPath() == setup.Path && requestBody == setup.Body {
				matched = true
				if setup.HTTPStatus == 0 {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(setup.HTTPStatus)
				}
				_, err := w.Write([]byte(setup.Response))
				if err != nil {
					t.Fatalf("Unable to write test server response: %v", err)
				}
			}
			/*t.Logf("No requests matched setup. Got method %s, Path %s, body %s\n wanted: method: %s path: %s body: %s\n", r.Method, r.URL.EscapedPath(), requestBody, setup.Method, setup.Path, setup.Body)
			t.Logf("%v Method: %v Path: %v Body: %v\n", matched, r.Method == setup.Method, r.URL.EscapedPath() == setup.Path, requestBody == setup.Body)
			if matched {
				t.Logf("No requests matched setup. Got method %s, Path %s, body %s\n wanted: method: %s path: %s body: %s\n", r.Method, r.URL.EscapedPath(), requestBody, setup.Method, setup.Path, setup.Body)
				t.Logf("%v Method: %v Path: %v Body: %v\n", matched, r.Method == setup.Method, r.URL.EscapedPath() == setup.Path, requestBody == setup.Body)
				break
			}*/
		}

		// TODO: remove before pushing
		/*if r.URL.EscapedPath() != setup.Path {
			t.Fatalf("wanted: %s got: %s\n", setup.Path, r.URL.EscapedPath())
		}*/
		if !matched {
			t.Fatalf("No requests matched setup. Got method %s, Path %s, body %s\n %s\n", r.Method, r.URL.EscapedPath(), requestBody, s)
		}
	})

	return httptest.NewServer(handlerFunc)
}
