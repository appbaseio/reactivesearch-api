package reindexer

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/appbaseio/arc/errors"
)

var testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
}))

func TestName(t *testing.T) {
	r := Instance()
	name := r.Name()
	if name != logTag {
		t.Errorf("unexpected plugin name, expected %s and got %s\n", logTag, name)
	}
}

var initTests = []struct {
	Instance *reindexer
	esURL    string
	expected error
}{
	{
		Instance(),
		testServer.URL,
		nil,
	},
	{
		Instance(),
		"",
		errors.NewEnvVarNotSetError(envEsURL),
	},
}

func TestInit(t *testing.T) {
	defer func() {
		testServer.Close()
		os.Clearenv()
	}()

	for _, it := range initTests {
		os.Setenv(envEsURL, it.esURL)
		actual := it.Instance.InitFunc()
		if !reflect.DeepEqual(actual, it.expected) {
			t.Errorf("got: %v want: %v\n", actual, it.expected)
		}
	}
}

func (r *reindexer) mockInitFunc() error {
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}

	client, err := newTestClient(url)
	if err != nil {
		return err
	}

	r.es = client
	return nil
}
