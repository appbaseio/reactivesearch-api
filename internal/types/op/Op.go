package op

import (
	"encoding/json"
	"errors"
)

type Operation int

const (
	Noop Operation = iota
	Read
	Write
	ReadWrite
)

func (o Operation) String() string {
	return [...]string{
		"noop",
		"read",
		"write",
		"read_write",
	}[o]
}

func (o *Operation) UnmarshalJSON(bytes []byte) error {
	var op string
	err := json.Unmarshal(bytes, &op)
	if err != nil {
		return err
	}
	switch op {
	case Noop.String():
		*o = Noop
	case Read.String():
		*o = Read
	case Write.String():
		*o = Write
	case ReadWrite.String():
		*o = ReadWrite
	default:
		return errors.New("invalid op encountered: " + op)
	}
	return nil
}

func (o Operation) MarshalJSON() ([]byte, error) {
	var op string
	switch o {
	case Noop:
		op = Noop.String()
	case Read:
		op = Read.String()
	case Write:
		op = Write.String()
	case ReadWrite:
		op = ReadWrite.String()
	default:
		return nil, errors.New("invalid op encountered: " + op)
	}
	return json.Marshal(op)
}
