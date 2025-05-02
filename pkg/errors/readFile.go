package errors

import "fmt"

type ErrReadFile struct {
	Err error
}

func (e ErrReadFile) Error() string {
	return fmt.Sprintf("read file: %v", e.Err)
}

func (e ErrReadFile) Unwrap() error {
	return e.Err
}

func NewReadFileError(err error) error {
	return ErrReadFile{Err: err}
}
