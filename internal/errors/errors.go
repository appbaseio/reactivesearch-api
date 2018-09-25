package errors

import "fmt"

type EnvVarNotSetError struct {
	Var string
}

func NewEnvVarNotSetError(varName string) *EnvVarNotSetError {
	return &EnvVarNotSetError{varName}
}

func (e *EnvVarNotSetError) Error() string {
	return fmt.Sprintf("%s env variable not set", e.Var)
}
