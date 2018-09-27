package op

import (
	"encoding/json"
	"errors"
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
		return errors.New("invalid op encountered: " + op)
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
		return nil, errors.New("invalid op encountered: " + op)
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
