package classifier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/appbaseio-confidential/arc/model/op"
)

func TestOpClassifier(t *testing.T) {
	tests := []struct {
		method     string
		expectedOp op.Operation
	}{
		{
			method:     http.MethodGet,
			expectedOp: op.Read,
		},
		{
			method:     http.MethodPut,
			expectedOp: op.Write,
		},
		{
			method:     http.MethodPost,
			expectedOp: op.Write,
		},
		{
			method:     http.MethodDelete,
			expectedOp: op.Delete,
		},
		{
			method:     http.MethodOptions,
			expectedOp: op.Read,
		},
		{
			method:     http.MethodHead,
			expectedOp: op.Read,
		},
		{
			method:     http.MethodPatch,
			expectedOp: op.Write,
		},
		{
			method:     http.MethodTrace,
			expectedOp: op.Write,
		},
		{
			method:     http.MethodConnect,
			expectedOp: op.Read,
		},
	}

	for _, test := range tests {
		handler := Instance().OpClassifier(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ctxOp := ctx.Value(op.CtxKey)
			if ctxOp == nil {
				t.Errorf("handler didn't receive *op.Operation in request context")
			}

			operation, ok := ctxOp.(*op.Operation)
			if !ok {
				t.Errorf("handler received incorrect op type: got %T expected *op.Operation", ctxOp)
			}

			if *operation != test.expectedOp {
				t.Errorf("op classified incorrectly for method %s: got '%v' expected '%v'",
					test.method, *operation, test.expectedOp)
			}
		})

		w := httptest.NewRecorder()
		r, err := http.NewRequest(test.method, "/", nil)
		if err != nil {
			t.Fatal(err)
		}

		handler.ServeHTTP(w, r)
	}
}
