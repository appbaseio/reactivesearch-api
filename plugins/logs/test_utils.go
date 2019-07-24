package logs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/olivere/elastic/v7"
)

func newStubClient(url, indexName string) (*elasticsearch, error) {
	client, err := elastic.NewSimpleClient(elastic.SetURL(url))
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic client: %v", err)
	}
	es := &elasticsearch{url, indexName, client}
	return es, nil
}

type ServerSetup struct {
	Method, Path, Body, Response string
	HTTPStatus                   int
}

// Taken from https://github.com/github/vulcanizer/blob/master/es_test.go
func buildTestServer(t *testing.T, setups []*ServerSetup) *httptest.Server {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, _ := ioutil.ReadAll(r.Body)
		requestBody := string(requestBytes)

		var s string
		matched := false
		for _, setup := range setups {
			// TODO: remove this
			s = setup.Method + ": " + setup.Path + ": " + setup.Body
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
		}

		if !matched {
			t.Fatalf("No requests matched setup. Got method %s, Path %s, body %q\n %q\n", r.Method, r.URL.EscapedPath(), requestBody, s)
		}
	})

	return httptest.NewServer(handlerFunc)
}

func compareErrs(expectedErr string, actual error) bool {
	if actual == nil {
		if expectedErr == "" {
			return true
		}
		return false
	}

	return expectedErr == actual.Error()
}
