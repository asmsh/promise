package promise

import (
	"errors"
	"fmt"
)

var (
	ErrPromiseConsumed = errors.New("promise already handled")

	ErrPromisePanicked = errors.New("promise panicked")

	// ErrPromiseNilResult will be returned when a callback returns nil as Result value,
	// when a callback calls panic with nil, or when a callback calls runtime.Goexit.
	// TODO: check if we should introduce a new error to report returns via runtime.Goexit or nil panic
	// quick way of returning typed Empty.
	//	ErrPromiseNilResult = errors.New("promise got nil as result")
)

// PanicError wraps a panic that got caught in a promise callback.
type PanicError struct {
	// V holds the value passed to the panic call.
	V any
}

func (e PanicError) Error() string {
	return fmt.Sprintf("panicked: %v", e.V)
}
