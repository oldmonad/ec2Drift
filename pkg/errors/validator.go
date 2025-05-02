package errors

import "fmt"

// ErrFormatValidation wraps a format validation error
type ErrFormatValidation struct {
	Err error
}

func (e ErrFormatValidation) Error() string {
	return fmt.Sprintf("format validation failed: %v", e.Err)
}

func (e ErrFormatValidation) Unwrap() error {
	return e.Err
}

func NewFormatValidationError(err error) error {
	return ErrFormatValidation{Err: err}
}

// ErrAttributeValidation wraps an attribute validation error
type ErrAttributeValidation struct {
	Err error
}

func (e ErrAttributeValidation) Error() string {
	return fmt.Sprintf("attribute validation failed: %v", e.Err)
}

func (e ErrAttributeValidation) Unwrap() error {
	return e.Err
}

func NewAttributeValidationError(err error) error {
	return ErrAttributeValidation{Err: err}
}

type InvalidAttributesError struct {
	InvalidAttrs []string
	ValidAttrs   []string
}

func (e *InvalidAttributesError) Error() string {
	var validFormatted string
	for _, attr := range e.ValidAttrs {
		validFormatted += fmt.Sprintf("  - %s\n", attr)
	}
	return fmt.Sprintf("invalid attributes: %v\nValid options:\n%s", e.InvalidAttrs, validFormatted)
}
