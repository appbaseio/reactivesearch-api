package users

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
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

		ctxACL := ctx.Value(acl.CtxKey)
		if ctxACL == nil {
			t.Errorf("*acl.ACL not in request context: got %v", ctxACL)
		}
		reqACL, ok := ctxACL.(*acl.ACL)
		if !ok {
			t.Errorf("cannot cast context acl to *acl.ACL: got %v", ctxACL)
		}
		if *reqACL != acl.User {
			t.Errorf("incorrect acl received in context: got %v expected %s", *reqACL, acl.User)
		}

		ctxOp := ctx.Value(op.CtxKey)
		if ctxOp == nil {
			t.Errorf("*op.Operation not in request context: got %v", ctxOp)
		}
		reqOp, ok := ctxOp.(*op.Operation)
		if !ok {
			t.Errorf("cannot cast context to *op.Operation: got %v", ctxOp)
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
	handler := classifier(testHandler)

	for _, test := range tests {
		req, err := http.NewRequest(test.method, "/_user", nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(rr, req)
	}
}
