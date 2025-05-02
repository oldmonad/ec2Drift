package errors

import "fmt"

// ErrServerListen wraps failures in ListenAndServe.
type ErrServerListen struct {
	Addr string
	Err  error
}

func (e ErrServerListen) Error() string {
	return fmt.Sprintf("server listen error at %q: %v", e.Addr, e.Err)
}

func (e ErrServerListen) Unwrap() error {
	return e.Err
}

func NewErrServerListen(addr string, err error) error {
	return ErrServerListen{Addr: addr, Err: err}
}

// ErrServerShutdown wraps failures in Shutdown.
type ErrServerShutdown struct {
	Err error
}

func (e ErrServerShutdown) Error() string {
	return fmt.Sprintf("shutdown error: %v", e.Err)
}

func (e ErrServerShutdown) Unwrap() error {
	return e.Err
}

func NewErrServerShutdown(err error) error {
	return ErrServerShutdown{Err: err}
}

// ErrInvalidJSON wraps JSON decode failures.
type ErrInvalidJSON struct {
	Err error
}

func (e ErrInvalidJSON) Error() string {
	return fmt.Sprintf("invalid JSON body: %v", e.Err)
}

func (e ErrInvalidJSON) Unwrap() error {
	return e.Err
}

func NewErrInvalidJSON(err error) error {
	return ErrInvalidJSON{Err: err}
}

// ErrAppRun wraps unexpected failures from the AppRunner.
type ErrAppRun struct {
	Err error
}

func (e ErrAppRun) Error() string {
	return fmt.Sprintf("failed to run drift check: %v", e.Err)
}

func (e ErrAppRun) Unwrap() error {
	return e.Err
}

func NewErrAppRun(err error) error {
	return ErrAppRun{Err: err}
}
