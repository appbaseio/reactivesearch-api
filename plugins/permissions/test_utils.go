package permissions

import (
	"os"

	"github.com/appbaseio/arc/util"
)

// TestURL for arc
var TestURL = "http://foo:bar@localhost:8000"

func newStubClient(url, indexName string) (*elasticsearch, error) {
	os.Setenv(envEsURL, TestURL)
	util.NewClient()
	es := &elasticsearch{
		indexName: indexName,
	}
	return es, nil
}
