package promise

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrPromisePanicked = errors.New("promise panicked")

	ErrPromiseConsumed = errors.New("promise already handled")

	// ErrPromiseNilCtxDone returned via [Result.Err] from [Group.Ctx] when the
	// [context.Context] value passed has a nil Done channel ([context.Context.Done]),
	// and the [GroupConfig.ErrorWhenCtxNilDone] [Group] option is set to `ture`.
	// This is a sync error that will happen before the [Promise] value is returned.
	ErrPromiseNilCtxDone = errors.New("promise got nil as result")

	// ErrPromiseGroupBusy returned via [Result.Err] from [Group.Chan], [Group.Delay],
	// and all [Group.Go] calls, when the [Group] is currently handling promises that
	// equals the [Group]'s assigned size ([GroupConfig.Size]),
	// and the [GroupConfig.ErrorWhenGroupBusy] [Group] option is set to `ture`.
	// This is a sync error that will happen before the [Promise] value is returned.
	ErrPromiseGroupBusy = errors.New("group is busy with other work")

	// ErrPromiseGroupDone returned when one of the [Group]'s Wait methods has
	// been called, and a new call is made to one of the [Group]'s Go methods.
	// This is a sync error that will happen before the [Promise] value is returned.
	ErrPromiseGroupDone = errors.New("group is done handling new work")
)

// PanicError wraps a panic that got caught in a promise callback.
type PanicError struct {
	// V holds the value passed to the panic call.
	V any
}

func (e PanicError) Error() string {
	return fmt.Sprintf("panicked: %v", e.V)
}
func (e PanicError) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (e PanicError) Unwrap() error {
	// try to return the panic value as an error value if it's really an error value.
	if err, ok := e.V.(error); ok {
		return err
	}
	return nil
}

// IdxError is the error container for an error returned from
// the Select extension call.
// It's also the container type for the MultiIdxError's elements.
type IdxError struct {
	Idx int
	Err error
}

func (e IdxError) Error() string {
	return fmt.Sprintf("[%d]%s", e.Idx, e.Err.Error())
}
func (e IdxError) Unwrap() error {
	return e.Err
}

// MultiIdxError is the error container for errors returned from
// the All(and AllWait), Any(and AnyWait) or Join extension calls.
type MultiIdxError struct {
	errs []error // always a []IdxError
}

func (e MultiIdxError) Error() string {
	if len(e.errs) == 1 {
		return e.errs[0].Error()
	}

	errb := strings.Builder{}
	for _, ee := range e.errs {
		if errb.Len() != 0 {
			errb.WriteByte('\n')
		}
		errb.WriteString(ee.Error())
	}
	return errb.String()
}

func (e MultiIdxError) Unwrap() []error {
	return e.errs
}
