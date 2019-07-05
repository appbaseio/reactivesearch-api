package permissions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/appbaseio/arc/errors"
	"github.com/appbaseio/arc/middleware"
)

var esTestServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(""))
}))

func TestName(t *testing.T) {
	p := Instance()
	name := p.Name()
	if name != logTag {
		t.Errorf("unexpected plugin name, expected %s and got %s\n", logTag, name)
	}
}

func TestRoutes(t *testing.T) {
	p := Instance()
	routes := p.Routes()
	// TODO: Add a better test
	if routes[0].Methods == nil {
		t.Fatalf("Invalid method")
	}
}

var InitTests = []struct {
	Instance  *permissions
	esURL     string
	permIndex string
	expected  error
}{
	{
		Instance(),
		esTestServer.URL,
		defaultPermissionsEsIndex,
		nil,
	},
	{
		Instance(),
		"",
		defaultPermissionsEsIndex,
		errors.NewEnvVarNotSetError(envEsURL),
	},
	{
		Instance(),
		esTestServer.URL,
		"",
		nil,
	},
	// invalid url to simulate a failure
	{
		Instance(),
		"elastic://localhost:9200/error",
		"",
		fmt.Errorf("[permissions]: error while initializing elastic client: health check timeout: no Elasticsearch node available"),
	},
}

func TestInit(t *testing.T) {
	defer func() {
		esTestServer.Close()
		os.Clearenv()
	}()

	for _, it := range InitTests {
		os.Setenv(envEsURL, it.esURL)
		os.Setenv(envPermissionEsIndex, it.permIndex)
		actual := it.Instance.InitFunc(make([]middleware.Middleware, 0))
		if !reflect.DeepEqual(actual, it.expected) {
			t.Errorf("got: %v want: %v\n", actual, it.expected)
		}
	}
}
