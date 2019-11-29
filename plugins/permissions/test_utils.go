package permissions

import (
	"os"

	"github.com/appbaseio/arc/util"
)

func newStubClient(url, indexName, mapping string) (*elasticsearch, error) {
	os.Setenv(envEsURL, "http://127.0.0.1:9200")
	util.NewClient()
	es := &elasticsearch{
		indexName: indexName,
		mapping:   mapping,
	}
	return es, nil
}
