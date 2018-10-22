package op

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio-confidential/arc/internal/errors"
)

type contextKey string

const CtxKey = contextKey("op")

type Operation int

const (
	Read Operation = iota
	Write
	Delete
)

func (o Operation) String() string {
	return [...]string{
		"read",
		"write",
		"delete",
	}[o]
}

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
		return nil, fmt.Errorf("invalid op encountered: %v", op)
	}
	return json.Marshal(op)
}

func Contains(slice []Operation, val Operation) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func FromContext(ctx context.Context) (*Operation, error) {
	ctxOp := ctx.Value(CtxKey)
	if ctxOp == nil {
		return nil, errors.NewNotFoundInRequestContextError("*op.Operation")
	}
	reqOp, ok := ctxOp.(*Operation)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxOp", "*op.Operation")
	}
	return reqOp, nil
}
