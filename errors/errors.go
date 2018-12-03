package errors

import (
	"errors"
	"fmt"
)

// Nil field errors.
var (
	ErrNilIndices    = errors.New(`arc: indices cannot be set to "nil"`)
	ErrNilACLs       = errors.New(`arc: acls cannot be set to "nil"`)
	ErrNilOps        = errors.New(`arc: ops cannot be set to "nil"`)
	ErrNilCategories = errors.New(`arc: categories cannot be set to "nil"`)
	ErrNilSources    = errors.New(`arc: sources cannot be set to "nil"`)
	ErrNilReferers   = errors.New(`arc: referers cannot be set to "nil"`)
)

// EnvVarNotSetError is an error which is returned when a required env var is not set.
type EnvVarNotSetError struct {
	Var string
}

// NewEnvVarNotSetError returns an error for an envVarName whose value is not set.
func NewEnvVarNotSetError(envVarName string) *EnvVarNotSetError {
	return &EnvVarNotSetError{envVarName}
}

// Error implements the error interface.
func (e *EnvVarNotSetError) Error() string {
	return fmt.Sprintf("arc: %s env variable not set", e.Var)
}

// UnsupportedPatchError is an error which is returned when a patch request is received for a readonly field.
type UnsupportedPatchError struct {
	Type  string
	Field string
}

// NewUnsupportedPatchError returns an error for a readonly field in a given type when it is tried to be modified.
func NewUnsupportedPatchError(typeName, field string) *UnsupportedPatchError {
	return &UnsupportedPatchError{typeName, field}
}

// Error implements the error interface.
func (u *UnsupportedPatchError) Error() string {
	return fmt.Sprintf("arc: cannot patch field %s in %s", u.Field, u.Type)
}

// NotFoundInContextError is an error which is returned when an expected value in the context is missing.
type NotFoundInContextError struct {
	Field string
}

// NewNotFoundInContextError returns an error for the given field when it is missing from the context.
func NewNotFoundInContextError(field string) *NotFoundInContextError {
	return &NotFoundInContextError{field}
}

// Error implements the error interface.
func (n *NotFoundInContextError) Error() string {
	return fmt.Sprintf("\"%s\" not found in request context", n.Field)
}

// InvalidCastError is an error which is returned when an invalid cast of a particular type is attempted.
type InvalidCastError struct {
	From string
	To   string
}

// NewInvalidCastError returns an error two types that were involved in invalid cast operation.
func NewInvalidCastError(from, to string) *InvalidCastError {
	return &InvalidCastError{from, to}
}

// Error implements the error interface.
func (i *InvalidCastError) Error() string {
	return fmt.Sprintf("cannot cast %s to %s", i.From, i.To)
}
