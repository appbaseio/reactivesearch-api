package op

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio-confidential/arc/errors"
)

type contextKey string

// ctxKey is a key against which an operation is stored in the context.
const ctxKey = contextKey("op")

// Operation defines an operation type.
type Operation int

// Operations
const (
	Read Operation = iota
	Write
	Delete
)

// String is the implementation of Stringer interface that returns the string representation of op.Action.
func (o Operation) String() string {
	return [...]string{
		"read",
		"write",
		"delete",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling op.Action type.
func (o *Operation) UnmarshalJSON(bytes []byte) error {
	var op string
	err := json.Unmarshal(bytes, &op)
	if err != nil {
		return err
	}
	switch op {
	case Read.String():
		*o = Read
	case Write.String():
		*o = Write
	case Delete.String():
		*o = Delete
	default:
		return fmt.Errorf("invalid op encountered: %v", op)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling op.Action type.
func (o Operation) MarshalJSON() ([]byte, error) {
	var op string
	switch o {
	case Read:
		op = Read.String()
	case Write:
		op = Write.String()
	case Delete:
		op = Delete.String()
	default:
		return nil, fmt.Errorf("invalid op encountered: %v", o)
	}
	return json.Marshal(op)
}

// NewContext returns a new context with the given Operation.
func NewContext(ctx context.Context, op *Operation) context.Context {
	return context.WithValue(ctx, ctxKey, op)
}

// FromContext retrieves the *op.Action stored against the op.CtxKey from the context.
func FromContext(ctx context.Context) (*Operation, error) {
	ctxOp := ctx.Value(ctxKey)
	if ctxOp == nil {
		return nil, errors.NewNotFoundInContextError("*op.Action")
	}
	reqOp, ok := ctxOp.(*Operation)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxOp", "*op.Action")
	}
	return reqOp, nil
}
