package errors

import (
	"fmt"
)

type CommandError struct {
	Err error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("command execution error: %v", e.Err)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}
