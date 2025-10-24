// Copyright 2024 Ahmad Sameh(asmsh)
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
	"fmt"
)

// internal result types
// the purpose of these types is to reduce allocations when setting the Result of
// the resolved promise, and implement the required logic to investigate the error
// structure, using the error, errors.Unwrap, errors.Is and errors.As interfaces.
// also to ensure consistent string conversion of the results.

func newSingleRes[T any, TElem Result[T]](s State, val TElem) Result[TElem] {
	switch s {
	case Success:
		return successResultSingleRes[T, TElem]{val}
	case Error:
		return errorResultSingleRes[T, TElem]{val}
	case Panic:
		return panicResultSingleRes[T, TElem]{val}
	default:
		// an internal panic, because it's supposed to be caught earlier.
		panic("promise: internal: unexpected Result state: " + s.String())
	}
}

// successResultSingleRes is a Result implementation for Success results
// returned from the Select extension call.
type successResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r successResultSingleRes[T, TElem]) Val() TElem   { return r.val }
func (r successResultSingleRes[T, TElem]) Err() error   { return nil }
func (r successResultSingleRes[T, TElem]) State() State { return Success }
func (r successResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	_, _ = fmt.Fprintf(f, "%s: %v", Success.String(), r.val)
}

// errorResultSingleRes is a Result and error implementation for Error
// results returned from the Select extension call.
type errorResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r errorResultSingleRes[T, TElem]) Val() TElem   { return r.val }
func (r errorResultSingleRes[T, TElem]) Err() error   { return r }
func (r errorResultSingleRes[T, TElem]) State() State { return Error }
func (r errorResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%s: %v", Error.String(), r.val)
}
func (r errorResultSingleRes[T, TElem]) Unwrap() error {
	switch res := any(r.val).(type) {
	case IdxRes[T]:
		return IdxError{Idx: res.Idx, Err: res.Err()}
	case GroupRes[T]:
		return GroupError{Err: res.Err()}
	}
	return nil
}

// panicResultSingleRes is a Result and error implementation for Panic
// results returned from the Select extension call.
type panicResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r panicResultSingleRes[T, TElem]) Val() (v TElem) { return r.val }
func (r panicResultSingleRes[T, TElem]) Err() error     { return r }
func (r panicResultSingleRes[T, TElem]) State() State   { return Panic }
func (r panicResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%s: %v", Panic.String(), r.val)
}
func (r panicResultSingleRes[T, TElem]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panicResultSingleRes[T, TElem]) Unwrap() error {
	switch res := any(r.val).(type) {
	case IdxRes[T]:
		return IdxError{Idx: res.Idx, Err: res.Err()}
	case GroupRes[T]:
		return GroupError{Err: res.Err()}
	}
	return nil
}
func (r panicResultSingleRes[T, TElem]) As(target any) bool {
	switch perr := target.(type) {
	case *PanicError:
		perr.V = r.PanicV()
		return true
	case *IdxError:
		if res, ok := any(r.val).(IdxRes[T]); ok {
			perr.Idx = res.Idx
			perr.Err = res.Err()
			return true
		}
	case *GroupError:
		if res, ok := any(r.val).(GroupRes[T]); ok {
			perr.Err = res.Err()
			return true
		}
	}

	// return on non-supported target types.
	return false
}
func (r panicResultSingleRes[T, TElem]) PanicV() any {
	return getPanicVFromRes(r.val)
}

func newMultiRes[T any, TElem Result[T]](s State, vals []TElem) Result[[]TElem] {
	switch s {
	case Panic:
		return panicResultMultiRes[T, TElem]{vals}
	case Error:
		return errorResultMultiRes[T, TElem]{vals}
	case Success:
		return successResultMultiRes[T, TElem]{vals}
	default:
		// an internal panic, because it's supposed to be caught earlier.
		panic("promise: internal: unexpected Result state: " + s.String())
	}
}

// errPromiseConsumedResult is a static error result that returns [ErrPromiseConsumed].
// it's used instead of saving the [ErrPromiseConsumed] error in a generic errResult value.
type errPromiseConsumedResult[T any] struct{}

func (r errPromiseConsumedResult[T]) Val() (v T)   { return v }
func (r errPromiseConsumedResult[T]) Err() error   { return ErrPromiseConsumed }
func (r errPromiseConsumedResult[T]) State() State { return Error }
func (r errPromiseConsumedResult[T]) String() string {
	return fmt.Sprintf("Error: %s", ErrPromiseConsumed.Error())
}

// errCtxNilDoneResult is a static error result that returns [ErrNilCtxDone].
// it's used instead of saving the [ErrNilCtxDone] error in a generic errResult value.
type errCtxNilDoneResult[T any] struct{}

func (r errCtxNilDoneResult[T]) Val() (v T)   { return v }
func (r errCtxNilDoneResult[T]) Err() error   { return ErrNilCtxDone }
func (r errCtxNilDoneResult[T]) State() State { return Error }
func (r errCtxNilDoneResult[T]) String() string {
	return fmt.Sprintf("Error: %s", ErrNilCtxDone.Error())
}

// errGroupBusyResult is a static error result that returns [ErrGroupBusy].
// it's used instead of saving the [ErrGroupBusy] error in a generic errResult value.
type errGroupBusyResult[T any] struct{}

func (r errGroupBusyResult[T]) Val() (v T)   { return v }
func (r errGroupBusyResult[T]) Err() error   { return ErrGroupBusy }
func (r errGroupBusyResult[T]) State() State { return Error }
func (r errGroupBusyResult[T]) String() string {
	return fmt.Sprintf("Error: %s", ErrGroupBusy.Error())
}

// errGroupWaitingResult is a static error result that returns [ErrGroupWaiting].
// it's used instead of saving the [ErrGroupWaiting] error in a generic errResult value.
type errGroupWaitingResult[T any] struct{}

func (r errGroupWaitingResult[T]) Val() (v T)   { return v }
func (r errGroupWaitingResult[T]) Err() error   { return ErrGroupWaiting }
func (r errGroupWaitingResult[T]) State() State { return Error }
func (r errGroupWaitingResult[T]) String() string {
	return fmt.Sprintf("Error: %s", ErrGroupWaiting.Error())
}

// errGroupCanceledResult is a static error result that returns [ErrGroupCanceled].
// it's used instead of saving the [ErrGroupCanceled] error in a generic errResult value.
type errGroupCanceledResult[T any] struct{}

func (r errGroupCanceledResult[T]) Val() (v T)   { return v }
func (r errGroupCanceledResult[T]) Err() error   { return ErrGroupCanceled }
func (r errGroupCanceledResult[T]) State() State { return Error }
func (r errGroupCanceledResult[T]) String() string {
	return fmt.Sprintf("Error: %s", ErrGroupCanceled.Error())
}
