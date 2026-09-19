package app

import "fmt"

type ErrorKind string

const (
	ErrInvalid     ErrorKind = "invalid"
	ErrAuth        ErrorKind = "authentication"
	ErrFetch       ErrorKind = "fetch"
	ErrNoTarget    ErrorKind = "no-target"
	ErrAmbiguous   ErrorKind = "ambiguous"
	ErrIncomplete  ErrorKind = "incomplete"
	ErrUnavailable ErrorKind = "unavailable"
	ErrExternal    ErrorKind = "external"
	ErrSave        ErrorKind = "save"
)

type AppError struct {
	Kind       ErrorKind
	Operation  string
	Profile    string
	Region     string
	InstanceID string
	Err        error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return string(e.Kind)
	}
	return fmt.Sprintf("%s: %v", e.Kind, e.Err)
}
func (e *AppError) Unwrap() error { return e.Err }
