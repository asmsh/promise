package promise

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrPromisePanicked is returned via [Result.Err] when a [Promise]'s callback
	// has caused a panic while being executed.
	// The panic value can be retrieved from the [PanicError.V] after type casting
	// the [Result.Err] to [PanicError] via [errors.As].
	//
	// All equality checks must be only done with the [errors.Is] function.
	// All type casting must be only done with the [errors.As] function.
	ErrPromisePanicked = errors.New("promise panicked")

	// ErrPromiseConsumed is returned via [Result.Err] when a [Promise]'s [Result]
	// is being accessed for the second or more time, and the [GroupConfig.OnetimeHandling]
	// flag is set to `true`.
	//
	// Accessing a [Promise]'s [Result] is done via [Promise.Res] or as an argument
	// to any other callback that accepts a [Result] value.
	//
	// All equality checks must be only done with the [errors.Is] function.
	ErrPromiseConsumed = errors.New("promise already handled")

	// ErrNilCtxDone is returned via [Result.Err] from [Group.Ctx] when
	// the [context.Context] value passed has a nil Done channel ([context.Context.Done]),
	// and the [GroupConfig.NoNilCtxDoneChan] flag is set to `true`.
	//
	// This is a synchronous error that will happen before the [Promise] value is returned.
	// All equality checks must be only done with the [errors.Is] function.
	ErrNilCtxDone = errors.New("nil Done chan found in Context")

	// ErrGroupBusy returned via [Result.Err] from a [Group]'s [Promise] constructor
	// when the number of promises the [Group] is currently handling equals the [GroupConfig.Size]
	// and the [GroupConfig.NoWaitingBusyGroup] flag is set to `true`.
	//
	// This is a synchronous error that will happen before the [Promise] value is returned.
	// All equality checks must be only done with the [errors.Is] function.
	//
	// The Group's Promise constructors are the [Group.Go] methods, the [Group.Delay],
	// [Group.Chan], and [Group.Ctx] methods
	ErrGroupBusy = errors.New("group is busy with other work")

	// ErrGroupWaiting is returned via [Result.Err] from a [Group]'s [Promise] constructor
	// when one of the [Group]'s Wait methods has been called.
	//
	// This is a synchronous error that will happen before the [Promise] value is returned.
	// All equality checks must be only done with the [errors.Is] function.
	//
	// The Group's Promise constructors are the [Group.Go] methods, the [Group.Delay],
	// [Group.Chan], and [Group.Ctx] methods
	//
	// The Group's Wait methods are the [Group.Wait], [Group.AllWaitRes],
	// [Group.AnyWaitRes], and [Group.JoinRes].
	ErrGroupWaiting = errors.New("group is waiting ongoing work")

	// ErrGroupCanceled is returned via [Result.Err] from a [Group]'s [Promise] constructor
	// if one of the [Group]'s previous promises produces an [Error] or a [Panic] [Result]
	// that's not handled.
	//
	// This is a synchronous error that will happen before the [Promise] value is returned.
	// All equality checks must be only done with the [errors.Is] function.
	//
	// Handling an [Error] [Result] is done via one of [Promise.Catch], [Promise.Res],
	// [Promise.Delay], [Promise.Follow] or [Promise.FollowCallback].
	//
	// Handling a [Panic] [Result] is done via one of [Promise.Recover], [Promise.Res],
	// [Promise.Delay], [Promise.Follow] or [Promise.FollowCallback].
	ErrGroupCanceled = errors.New("group is canceled")
)

// PanicError wraps a panic that got caught in a promise callback.
type PanicError struct {
	// V holds the value passed to the panic call.
	V any
}

func (e PanicError) Error() string {
	return fmt.Sprintf("Panic: %v", e.V)
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
// the [Select], [All](and [AllWait]), [Any](and [AnyWait]),
// or [Join] extension calls.
// It's also one of the container types for the MultiError's elements.
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

// GroupError is the error container for an error returned from
// the [Group.Select], [Group.AllRes](and [Group.AllWaitRes]),
// [Group.AnyRes](and [Group.AnyWaitRes]), or [Group.Join] group calls.
// It's also one of the container types for the MultiError's elements.
type GroupError struct {
	Err error
}

func (e GroupError) Error() string {
	return e.Err.Error()
}
func (e GroupError) Unwrap() error {
	return e.Err
}

// MultiError is the error container for errors returned from the
// [All](and [AllWait]), [Any](and [AnyWait]) or [Join] extension calls,
// and the [Group.AllWaitRes] or [Group.AnyWaitRes] group calls.
type MultiError struct {
	errs []error // either a [][IdxError] or [][GroupError]
}

func (e MultiError) Error() string {
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

func (e MultiError) Unwrap() []error {
	return e.errs
}
