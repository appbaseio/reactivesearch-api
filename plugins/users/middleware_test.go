package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/user"
)

func TestClassifier(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{
			method: http.MethodGet,
			path:   "/_user",
		},
		{
			method: http.MethodPut,
			path:   "/_user",
		},
		{
			method: http.MethodPost,
			path:   "/_user",
		},
		{
			method: http.MethodPatch,
			path:   "/_user",
		},
		{
			method: http.MethodDelete,
			path:   "/_user",
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			t.Errorf("%v", err)
		}
		if *reqCategory != category.User {
			t.Errorf("incorrect category received in context: got %v expected %s", *reqCategory, category.User)
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			t.Errorf("%v", err)
		}

		switch r.Method {
		case http.MethodGet:
			if *reqOp != op.Read {
				t.Errorf("incorrect op received for %s in context: got %v expected %s",
					http.MethodGet, *reqOp, op.Read)
			}
		case http.MethodPut:
			if *reqOp != op.Write {
				t.Errorf("incorrect op received for %s in context: got %v expected %s",
					http.MethodPut, *reqOp, op.Write)
			}
		case http.MethodPatch:
			if *reqOp != op.Write {
				t.Errorf("incorrect op received for %s in context: got %v expected %s",
					http.MethodPatch, *reqOp, op.Write)
			}
		case http.MethodDelete:
			if *reqOp != op.Delete {
				t.Errorf("incorrect op received for %s in context: got %v expected %s",
					http.MethodDelete, *reqOp, op.Delete)
			}
		case http.MethodPost:
			if *reqOp != op.Write {
				t.Errorf("incorrect op received for %s in context: got %v expected %s",
					http.MethodPost, *reqOp, op.Write)
			}
		default:
			t.Errorf("unsupported method")
		}
	})

	rr := httptest.NewRecorder()
	handler := classifyCategory(testHandler)

	for _, test := range tests {
		req, err := http.NewRequest(test.method, test.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)
	}
}

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		description  string
		opts         user.Options
		expectedCode int
	}{
		{
			description:  "user is an admin",
			opts:         user.SetIsAdmin(true),
			expectedCode: http.StatusOK,
		},
		{
			description:  "user is not an admin",
			opts:         user.SetIsAdmin(false),
			expectedCode: http.StatusUnauthorized,
		},
		{
			description:  "isAdmin middleware didn't receive *user.User",
			expectedCode: http.StatusInternalServerError,
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, test := range tests {
		req, err := http.NewRequest(http.MethodGet, "/_user", nil)
		if err != nil {
			t.Fatal(err)
		}
		if test.opts != nil {
			foobar, err := user.New("foo", "bar", test.opts)
			if err != nil {
				t.Fatal(err)
			}
			req = req.WithContext(context.WithValue(req.Context(), user.CtxKey, foobar))
		}

		handler := isAdmin(testHandler)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != test.expectedCode {
			t.Errorf("incorrect status code: got %d expected %d", rec.Code, test.expectedCode)
		}
	}
}
