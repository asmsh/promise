// Copyright 2026 Ahmad Sameh(asmsh)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promise

import (
	"errors"
	"fmt"
	"strings"
)

// All the different errors below must be checked with the [errors.Is] function,
// and any type casting must only be done via the [errors.As] function.
var (
	// ErrPromisePanicked is returned via [Result.Err] when a [Promise]'s callback
	// has caused a panic while being executed.
	// The panic value can be retrieved from the [PanicError.V] after type casting
	// the [Result.Err] to [PanicError] via [errors.As].
	//
	// All equality checks must be only done with the [errors.Is] function.
	// All type casting must be only done with the [errors.As] function.
	ErrPromisePanicked = errors.New("promise panicked")

	// ErrPromiseGoexit is returned via [Result.Err] when a [Promise]'s callback
	// calls [runtime.Goexit] while being executed.
	//
	// All equality checks must be only done with the [errors.Is] function.
	// All type casting must be only done with the [errors.As] function.
	ErrPromiseGoexit = errors.New("promise called runtime.Goexit")

	// ErrPromiseConsumed is returned via [Result.Err] when a [Promise]'s [Result]
	// is being accessed for the second or more time, and the [GroupConfig.OnetimeHandling]
	// flag is set to `true`.
	//
	// Accessing a [Promise]'s [Result] is done via [Promise.WaitRes] or as an argument
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
	// Handling an [Error] [Result] is done via one of [Promise.WaitRes],
	// [Promise.Delay], [Promise.Follow] or [Promise.Callback].
	//
	// Handling a [Panic] [Result] is done via one of [Promise.WaitRes],
	// [Promise.Delay], [Promise.Follow] or [Promise.Callback].
	ErrGroupCanceled = errors.New("group is canceled")
)

// PanicError wraps a recovered panic from a promise callback in an error value.
type PanicError struct {
	// V holds the value returned from recover.
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

// PanicV returns the underlying panic value wrapped by this error.
func (e PanicError) PanicV() any { return e.V }

// IdxError is the error container for an error returned from
// the [Select], [All](and [AllWait]), [Any](and [AnyWait]),
// or [Join] extension calls.
// It's also one of the container types for the MultiError's elements.
type IdxError struct {
	Idx int
	Err error
}

func (e IdxError) Error() string {
	return fmt.Sprintf("%s", e.Err.Error())
}
func (e IdxError) Unwrap() error {
	return e.Err
}

// GroupError is the error container for an error returned from
// the [Group.SelectRes], [Group.AllRes](and [Group.AllWaitRes]),
// [Group.AnyRes](and [Group.AnyWaitRes]), or [Group.JoinRes] group calls.
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
	Errs []error // either a [][IdxError] or [][GroupError]
}

func (e MultiError) Error() string {
	if len(e.Errs) == 1 {
		return e.Errs[0].Error()
	}

	errb := strings.Builder{}
	for _, ee := range e.Errs {
		if errb.Len() != 0 {
			errb.WriteByte('\n')
		}
		errb.WriteString(ee.Error())
	}
	return errb.String()
}

func (e MultiError) Unwrap() []error {
	return e.Errs
}
