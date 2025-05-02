package errors

import "fmt"

// ErrEnvLoad wraps failures loading the .env file.
type ErrEnvLoad struct {
	Err error
}

func (e ErrEnvLoad) Error() string {
	return fmt.Sprintf("error loading .env: %v", e.Err)
}

func (e ErrEnvLoad) Unwrap() error {
	return e.Err
}

func NewErrEnvLoad(err error) error {
	return ErrEnvLoad{Err: err}
}

// ErrConfigSetup wraps failures in SetupConfigurations.
type ErrConfigSetup struct {
	Err error
}

func (e ErrConfigSetup) Error() string {
	return fmt.Sprintf("error loading configuration: %v", e.Err)
}

func (e ErrConfigSetup) Unwrap() error {
	return e.Err
}

func NewErrConfigSetup(err error) error {
	return ErrConfigSetup{Err: err}
}
