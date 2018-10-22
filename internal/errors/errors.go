package errors

import (
	"errors"
	"fmt"
)

var (
	NilIndicesError = errors.New("indices cannot be set 'nil'")
	NilACLsError    = errors.New("acls cannot be set 'nil'")
	NilOpsError     = errors.New("ops cannot be set 'nil'")
)

type EnvVarNotSetError struct {
	Var string
}

func NewEnvVarNotSetError(varName string) *EnvVarNotSetError {
	return &EnvVarNotSetError{varName}
}

func (e *EnvVarNotSetError) Error() string {
	return fmt.Sprintf("%s env variable not set", e.Var)
}

type UnsupportedPatchError struct {
	Type  string
	Field string
}

func NewUnsupportedPatchError(typeName, field string) *UnsupportedPatchError {
	return &UnsupportedPatchError{typeName, field}
}

func (u *UnsupportedPatchError) Error() string {
	return fmt.Sprintf("cannot patch field %s in %s", u.Field, u.Type)
}

type MissingFieldError struct {
	Type  string
	Field string
}

func NewMissingFieldError(typeName string, field string) *MissingFieldError {
	return &MissingFieldError{typeName, field}
}

func (m *MissingFieldError) Error() string {
	return fmt.Sprintf("missing field %s for type %s", m.Field, m.Type)
}

type NotFoundInRequestContextError struct {
	Field string
}

func NewNotFoundInRequestContextError(field string) *NotFoundInRequestContextError {
	return &NotFoundInRequestContextError{field}
}

func (n *NotFoundInRequestContextError) Error() string {
	return fmt.Sprintf("%s not found in request context", n.Field)
}

type InvalidCastError struct {
	From string
	To   string
}

func NewInvalidCastError(from, to string) *InvalidCastError {
	return &InvalidCastError{from, to}
}

func (i *InvalidCastError) Error() string {
	return fmt.Sprintf("cannot cast %s to %s", i.From, i.To)
}
