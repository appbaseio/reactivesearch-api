package logs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/appbaseio/arc/errors"
)

var esTestServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("test ES server"))
}))

func TestName(t *testing.T) {
	l := Instance()
	name := l.Name()
	if name != logTag {
		t.Errorf("unexpected plugin name, expected %s and got %s\n", logTag, name)
	}
}

func TestRoutes(t *testing.T) {
	l := Instance()
	routes := l.Routes()
	// TODO: Add a better test
	if routes[0].Methods == nil {
		t.Fatalf("Invalid method")
	}
}

var InitTests = []struct {
	instance    *Logs
	esURL       string
	esLogsIndex string
	expected    error
}{
	{
		Instance(),
		esTestServer.URL,
		defaultLogsEsIndex,
		nil,
	},
	{
		Instance(),
		"",
		defaultLogsEsIndex,
		errors.NewEnvVarNotSetError(envEsURL),
	},
}

func TestInit(t *testing.T) {
	defer func() {
		esTestServer.Close()
		os.Clearenv()
	}()

	for _, it := range InitTests {
		os.Setenv(envEsURL, it.esURL)
		os.Setenv(envLogsEsIndex, it.esLogsIndex)
		actual := it.instance.InitFunc()
		if !reflect.DeepEqual(actual, it.expected) {
			t.Errorf("got: %v want: %v\n", actual, it.expected)
		}
	}
}

func (l *Logs) mockInitFunc() error {
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}

	index := os.Getenv(envLogsEsIndex)
	if index == "" {
		index = defaultLogsEsIndex
	}

	client, err := newStubClient(url, index)
	if err != nil {
		return err
	}

	l.es = client
	return nil
}
