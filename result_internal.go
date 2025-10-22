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
	"slices"
	"strings"
)

// internal result types
// the purpose of these types is to reduce allocations when setting the Result of
// the resolved promise, and implement the required logic to investigate the error
// structure, using the error, errors.Unwrap, errors.Is and errors.As interfaces.
// also to ensure consistent string conversion of the results.

func newSingleRes[T any, TElem Result[T]](s State, val TElem) Result[TElem] {
	switch s {
	case Panic:
		return panicResultSingleRes[T, TElem]{val}
	case Error:
		return errorResultSingleRes[T, TElem]{val}
	case Success:
		return successResultSingleRes[T, TElem]{val}
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
	printSingleRes(r.State().String(), r.val, f, verb)
}

// errorResultSingleRes is a Result and error implementation for Error
// results returned from the Select extension call.
type errorResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r errorResultSingleRes[T, TElem]) Val() TElem   { return r.val }
func (r errorResultSingleRes[T, TElem]) Err() error   { return r }
func (r errorResultSingleRes[T, TElem]) State() State { return Error }
func (r errorResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	printSingleRes(r.State().String(), r.val, f, verb)
}
func (r errorResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%s: %v", r.State().String(), r.val)
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
func (r panicResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	printSingleRes(r.State().String(), r.val, f, verb)
}
func (r panicResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%s: %v", r.State().String(), r.val)
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

// successResultMultiRes is a Result implementation for Success results
// returned from the All(and AllWait), Any(and AnyWait) or Join extension calls.
type successResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r successResultMultiRes[T, TElem]) Val() []TElem { return r.vals }
func (r successResultMultiRes[T, TElem]) Err() error   { return nil }
func (r successResultMultiRes[T, TElem]) State() State { return Success }
func (r successResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(r.State().String(), r.vals, f, verb)
}

// errorResultMultiRes is a Result and error implementation for Error
// results returned from the All(and AllWait), Any(and AnyWait) or Join
// extension calls.
type errorResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r errorResultMultiRes[T, TElem]) Val() []TElem { return r.vals }
func (r errorResultMultiRes[T, TElem]) Err() error   { return r }
func (r errorResultMultiRes[T, TElem]) State() State { return Error }
func (r errorResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(r.State().String(), r.vals, f, verb)
}
func (r errorResultMultiRes[T, TElem]) Error() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %v", r.State().String(), r.vals[0])
	}

	// find the first Error error and print it before any other errors.
	fi, fr := r.firstErrorRes()

	errb := &strings.Builder{}
	fmt.Fprintf(errb, "%s: %v", r.State().String(), fr)

	for i, ir := range r.vals {
		// not Error, or already printed
		if ir.State() != Error || i == fi {
			continue
		}
		fmt.Fprintf(errb, "\n%v", ir)
	}
	return errb.String()
}
func (r errorResultMultiRes[T, TElem]) Unwrap() []error {
	errs := make([]error, 0, len(r.vals))
	for _, ir := range r.vals {
		switch res := any(ir).(type) {
		case IdxRes[T]:
			// only [Error] is returned
			if res.Result == nil || res.State() != Error {
				continue
			}

			errs = append(errs, IdxError{Idx: res.Idx, Err: res.Err()})
		case GroupRes[T]:
			if res.Result == nil || res.State() != Error {
				continue
			}

			errs = append(errs, GroupError{Err: res.Err()})
		}
	}
	return errs
}
func (r errorResultMultiRes[T, TElem]) As(target any) bool {
	switch perr := target.(type) {
	case *IdxError:
		_, fr := r.firstErrorRes()
		if res, ok := any(fr).(IdxRes[T]); ok {
			perr.Idx = res.Idx
			perr.Err = res.Err()
			return true
		}
	case *GroupError:
		_, fr := r.firstErrorRes()
		if res, ok := any(fr).(GroupRes[T]); ok {
			perr.Err = res.Err()
			return true
		}
	case *MultiError:
		// include all errors.
		perr.errs = r.Unwrap()
		return true
	}

	// return on non-supported target types.
	return false
}
func (r errorResultMultiRes[T, TElem]) firstErrorRes() (int, TElem) {
	// find the first Error Result.
	// there must be at least one Result value, otherwise the whole result value
	// wouldn't have been created, and that value has to be with Error State.
	i := slices.IndexFunc(r.vals, func(ir TElem) bool { return ir.State() == Error })
	return i, r.vals[i]
}

// panicResultMultiRes is a Result and error implementation for panic
// results returned from either the All(and AllWait), Any(and AnyWait) or Join
// extension calls.
type panicResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r panicResultMultiRes[T, TElem]) Val() []TElem { return r.vals }
func (r panicResultMultiRes[T, TElem]) Err() error   { return r }
func (r panicResultMultiRes[T, TElem]) State() State { return Panic }
func (r panicResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(r.State().String(), r.vals, f, verb)
}
func (r panicResultMultiRes[T, TElem]) Error() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %v", r.State().String(), r.vals[0])
	}

	// find the first Panic error and print it before any other errors.
	fi, fr := r.firstPanicRes()

	errb := &strings.Builder{}
	fmt.Fprintf(errb, "%s: %v", r.State().String(), fr)

	for i, ir := range r.vals {
		// not Error nor Panic, or already printed
		if ir.State() == Success || i == fi {
			continue
		}
		fmt.Fprintf(errb, "\n%v", ir)
	}
	return errb.String()
}
func (r panicResultMultiRes[T, TElem]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panicResultMultiRes[T, TElem]) Unwrap() []error {
	errs := make([]error, 0, len(r.vals))
	for _, ir := range r.vals {
		switch res := any(ir).(type) {
		case IdxRes[T]:
			// only [Panic] and [Error] are returned
			if res.Result == nil || res.State() == Success {
				continue
			}

			errs = append(errs, IdxError{Idx: res.Idx, Err: res.Err()})
		case GroupRes[T]:
			if res.Result == nil || res.State() == Success {
				continue
			}

			errs = append(errs, GroupError{Err: res.Err()})
		}
	}
	return errs
}
func (r panicResultMultiRes[T, TElem]) As(target any) bool {
	switch perr := target.(type) {
	case *PanicError:
		perr.V = r.PanicV()
	case *IdxError:
		_, fr := r.firstPanicRes()
		if res, ok := any(fr).(IdxRes[T]); ok {
			perr.Idx = res.Idx
			perr.Err = res.Err()
			return true
		}
	case *GroupError:
		_, fr := r.firstPanicRes()
		if res, ok := any(fr).(GroupRes[T]); ok {
			perr.Err = res.Err()
			return true
		}
	case *MultiError:
		// include all errors.
		perr.errs = r.Unwrap()
		return true
	}

	// return on non-supported target types.
	return false
}
func (r panicResultMultiRes[T, TElem]) PanicV() any {
	_, fr := r.firstPanicRes()
	return getPanicVFromRes(fr)
}
func (r panicResultMultiRes[T, TElem]) firstPanicRes() (int, TElem) {
	// find the first Panic Result.
	// there must be at least one Result value, otherwise the whole result value
	// wouldn't have been created, and that value has to be with Panic State.
	i := slices.IndexFunc(r.vals, func(ir TElem) bool { return ir.State() == Panic })
	return i, r.vals[i]
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

func printSingleRes[T any, TElem Result[T]](
	stateStr string,
	val TElem,
	f fmt.State,
	verb rune,
) {
	fmt.Fprintf(f, "%s: %v", stateStr, val)
}

func printMultiRes[T any, TElem Result[T]](
	stateStr string,
	vals []TElem,
	f fmt.State,
	verb rune,
) {
	if len(vals) == 0 {
		fmt.Fprintf(f, "%s: nil", stateStr)
		return
	}
	if len(vals) == 1 {
		fmt.Fprintf(f, "%s: %v", stateStr, vals[0])
		return
	}

	f.Write([]byte(stateStr))
	f.Write([]byte(": "))

	for idx, ir := range vals {
		if idx == 0 {
			fmt.Fprintf(f, "%v", ir)
		} else {
			fmt.Fprintf(f, "\n%v", ir)
		}
	}
}
