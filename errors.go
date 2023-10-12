package promise

import (
	"errors"
	"fmt"
)

var (
	ErrPromiseConsumed = errors.New("promise already handled")

	// ErrPromiseNilResult will be returned when a callback returns nil as Result value,
	// when a callback calls panic with nil, or when a callback calls runtime.Goexit.
	// TODO: check if we should introduce a new error to report returns via runtime.Goexit or nil panic
	// quick way of returning typed Empty.
	//	ErrPromiseNilResult = errors.New("promise got nil as result")
)

// UncaughtPanic wraps a panic that happened in a promise chain, but hasn't
// been caught, by the end of that chain.
// uncaughtPanic wraps a panic value that happened in a promise chain,
// but hasn't been caught by the end of that chain.
type UncaughtPanic struct{ v any }

func (e UncaughtPanic) Error() string {
	return fmt.Sprintf("uncaught panic in the promise chain: %v", e.v)
}

func (e UncaughtPanic) V() any { return e.v }

// UncaughtError wraps an error that happened in a promise chain, but hasn't
// been caught, by the end of that chain.
type UncaughtError struct{ err error }

func (e UncaughtError) Error() string {
	return fmt.Sprintf("uncaught error in the promise chain: %s", e.err)
}

func (e UncaughtError) Unwrap() error { return e.err }
