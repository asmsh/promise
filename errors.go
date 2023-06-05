package promise

import (
	"errors"
	"fmt"
)

var (
	ErrPromiseTimeout   = errors.New("promise: timeout")
	ErrPromiseConsumed  = errors.New("promise: already handled")
	ErrPromiseNilResult = errors.New("promise: got nil as result")
)

// UncaughtPanic wraps a panic that happened in a promise chain, but hasn't
// been caught, by the end of that chain.
// uncaughtPanic wraps a panic value that happened in a promise chain,
// but hasn't been caught, by the end of that chain.
type UncaughtPanic struct {
	v any
}

func (e *UncaughtPanic) Error() string {
	return fmt.Sprintf("uncaught panic in the promise chain: %v", e.v)
}

func (e *UncaughtPanic) V() any {
	return e.v
}

func newUncaughtPanic(v any) *UncaughtPanic {
	return &UncaughtPanic{v: v}
}

// UncaughtError wraps an error that happened in a promise chain, but hasn't
// been caught, by the end of that chain.
// TODO: is this really needed ?
//	 error returns will be returning the actual error.
//	 so far, it's only needed whenever panicking for uncaught error.
type UncaughtError struct {
	err error
}

func (e *UncaughtError) Error() string {
	return fmt.Sprintf("uncaught error in the promise chain: %s", e.err)
}

func (e *UncaughtError) Unwrap() error {
	return e.err
}

func newUncaughtError(err error) *UncaughtError {
	return &UncaughtError{err: err}
}
