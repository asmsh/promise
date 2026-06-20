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
	"errors"
	"fmt"
	"slices"
	"strings"
)

// internal result types
// the purpose of these types is to reduce allocations when setting the Result of
// the resolved promise, and implement the required logic to investigate the error
// structure, using the error, errors.Unwrap, errors.Is and errors.As interfaces.
// also to ensure consistent string conversion of the results.

// errPromiseGoexitResult is a static error result that returns [ErrPromiseGoexit].
// it's used instead of saving the [ErrPromiseGoexit] error in a generic errResult value.
type errPromiseGoexitResult[T any] struct{}

func (r errPromiseGoexitResult[T]) Val() (v T)   { return v }
func (r errPromiseGoexitResult[T]) Err() error   { return ErrPromiseGoexit }
func (r errPromiseGoexitResult[T]) State() State { return Error }
func (r errPromiseGoexitResult[T]) String() string {
	return fmt.Sprintf("Error: %s", ErrPromiseGoexit.Error())
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

// result functions and types for single wrapped Result values (IdxRes, GroupRes).

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
	printSingleRes(r, Success, r.val, f, verb)
}

// errorResultSingleRes is a Result and error implementation for Error
// results returned from the Select extension call.
type errorResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r errorResultSingleRes[T, TElem]) Val() (v TElem) { return v }
func (r errorResultSingleRes[T, TElem]) Err() error     { return r }
func (r errorResultSingleRes[T, TElem]) State() State   { return Error }
func (r errorResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	printSingleRes(r, Error, r.val, f, verb)
}
func (r errorResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%v", r.val)
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
func (r errorResultSingleRes[T, TElem]) Is(target error) bool {
	// saves the allocations in Unwrap when calling errors.Is.
	return errors.Is(r.val.Err(), target)
}
func (r errorResultSingleRes[T, TElem]) As(target any) bool {
	// saves the allocations in Unwrap when calling errors.As.
	switch perr := target.(type) {
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

// panicResultSingleRes is a Result and error implementation for Panic
// results returned from the Select extension call.
type panicResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r panicResultSingleRes[T, TElem]) Val() (v TElem) { return v }
func (r panicResultSingleRes[T, TElem]) Err() error     { return r }
func (r panicResultSingleRes[T, TElem]) State() State   { return Panic }
func (r panicResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	printSingleRes(r, Panic, r.val, f, verb)
}
func (r panicResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%v", r.val)
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
func (r panicResultSingleRes[T, TElem]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	if target == ErrPromisePanicked {
		return true
	}
	// saves the allocations in Unwrap when calling errors.Is.
	return errors.Is(r.val.Err(), target)
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

func printSingleRes[T any, TElem Result[T]](
	cont any,
	state State,
	val TElem,
	f fmt.State,
	verb rune,
) {
	_, _ = f.Write([]byte(state.String()))
	_, _ = f.Write([]byte(": "))
	_, _ = fmt.Fprintf(f, "%v", val)

	if verb == 'v' && f.Flag('#') {
		_, _ = fmt.Fprintf(f, "(%T)", cont)
	}
}

// result functions and types for multi wrapped Result values ([]IdxRes, []GroupRes).

func newMultiRes[T any, TElem Result[T]](
	op joinOperationLogic,
	s State,
	vals []TElem,
) Result[[]TElem] {
	if op == joinOp {
		return joinResultMultiRes[T, TElem]{vals: vals}
	}

	switch s {
	case Success:
		return successResultMultiRes[T, TElem]{vals}
	case Error:
		return errorResultMultiRes[T, TElem]{vals}
	case Panic:
		return panicResultMultiRes[T, TElem]{vals}
	default:
		// an internal panic, because it's supposed to be caught earlier.
		panic("promise: internal: unexpected Result state: " + s.String())
	}
}

// joinResultMultiRes is a [Result] implementation for [Join] calls.
//
// it's also used for results returned from extension or group calls.
type joinResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r joinResultMultiRes[T, TElem]) Val() []TElem {
	if len(r.vals) == 0 {
		return nil
	}
	return r.vals
}
func (r joinResultMultiRes[T, TElem]) Err() error   { return nil }
func (r joinResultMultiRes[T, TElem]) State() State { return Success }
func (r joinResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(true, r, Success, r.vals, f, verb)
}

// successResultMultiRes is a [Result] implementation for [Success] values
// returned from [MultiRes].
//
// it's also used for results returned from extension or group calls.
type successResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r successResultMultiRes[T, TElem]) Val() []TElem {
	valsLen := len(r.vals) // assuming all are Success vals.
	if valsLen == 0 {
		return nil
	}
	vals := make([]TElem, 0, valsLen)
	vals = getFilteredMultiResVals(Success, vals, r.vals)
	if len(vals) == 0 {
		return nil
	}
	return vals
}
func (r successResultMultiRes[T, TElem]) Err() error   { return nil }
func (r successResultMultiRes[T, TElem]) State() State { return Success }
func (r successResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(false, r, Success, r.vals, f, verb)
}

// errorResultMultiRes is a [Result] implementation for [Error] values
// returned from [MultiRes].
//
// it's also used for results returned from extension or group calls.
type errorResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r errorResultMultiRes[T, TElem]) Val() []TElem {
	valsLen := len(r.vals) - 1 // assuming only one non-Success val.
	if valsLen <= 0 {
		return nil
	}
	vals := make([]TElem, 0, valsLen)
	vals = getFilteredMultiResVals(Success, vals, r.vals)
	if len(vals) == 0 {
		return nil
	}
	return vals
}
func (r errorResultMultiRes[T, TElem]) Err() error   { return r }
func (r errorResultMultiRes[T, TElem]) State() State { return Error }
func (r errorResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(false, r, Error, r.vals, f, verb)
}
func (r errorResultMultiRes[T, TElem]) Error() string {
	return getMultiResErrorString(Error, r.vals)
}
func (r errorResultMultiRes[T, TElem]) Unwrap() []error {
	errs := make([]error, 0, len(r.vals)) // assuming all values are errors.
	errs = unwrapMultiResErrors(Error, r.vals, errs)
	return errs
}
func (r errorResultMultiRes[T, TElem]) As(target any) bool {
	return castMultiResAsError(Error, r.vals, target)
}

// panicResultMultiRes is a [Result] implementation for [Panic] values
// returned from [MultiRes].
//
// it's also used for results returned from extension or group calls.
type panicResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r panicResultMultiRes[T, TElem]) Val() []TElem {
	valsLen := len(r.vals) - 1 // assuming only one non-Success val.
	if valsLen <= 0 {
		return nil
	}
	vals := make([]TElem, 0, valsLen)
	vals = getFilteredMultiResVals(Success, vals, r.vals)
	if len(vals) == 0 {
		return nil
	}
	return vals
}
func (r panicResultMultiRes[T, TElem]) Err() error   { return r }
func (r panicResultMultiRes[T, TElem]) State() State { return Panic }
func (r panicResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(false, r, Panic, r.vals, f, verb)
}
func (r panicResultMultiRes[T, TElem]) Error() string {
	return getMultiResErrorString(Panic, r.vals)
}
func (r panicResultMultiRes[T, TElem]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panicResultMultiRes[T, TElem]) Unwrap() []error {
	errs := make([]error, 0, len(r.vals)) // assuming all values are errors.
	errs = unwrapMultiResErrors(Panic, r.vals, errs)
	return errs
}
func (r panicResultMultiRes[T, TElem]) As(target any) bool {
	return castMultiResAsError(Panic, r.vals, target)
}
func (r panicResultMultiRes[T, TElem]) PanicV() any {
	_, fr := getFirstRes(Panic, r.vals)
	return getPanicVFromRes(fr)
}

// helper functions for multi wrapped Result values ([]IdxRes, []GroupRes).

// filterResultFunc returns true when the got State is matching the target State.
// target Success matches with got Success only.
// target Error matches with got Error only.
// target Panic matches with either got Error or Panic.
func filterResultFunc(got State, target State) bool {
	if target == Success {
		return got == Success
	}
	if target == Error {
		return got == Error
	}
	if target == Panic {
		return got == Error || got == Panic
	}

	return false
}

func getFilteredMultiResVals[T any, TElem Result[T]](
	state State,
	dstVals []TElem,
	vals []TElem,
) []TElem {
	for _, v := range vals {
		if got := v.State(); filterResultFunc(got, state) {
			dstVals = append(dstVals, v)
		}
	}
	return dstVals
}

func printMultiRes[T any, TElem Result[T]](
	includeAll bool,
	cont any,
	state State,
	vals []TElem,
	f fmt.State,
	verb rune,
) {
	defer func() {
		if verb == 'v' && f.Flag('#') {
			_, _ = fmt.Fprintf(f, "(%T)", cont)
		}
	}()

	if len(vals) == 0 {
		_, _ = fmt.Fprintf(f, "%s: nil", state.String())
		return
	}
	if len(vals) == 1 {
		_, _ = fmt.Fprintf(f, "%s: %v", state.String(), vals[0])
		return
	}

	_, _ = f.Write([]byte(state.String()))
	_, _ = f.Write([]byte(": "))

	started := false
	for _, v := range vals {
		if includeAll || filterResultFunc(v.State(), state) {
			if !started {
				_, _ = fmt.Fprintf(f, "%v", v)
			} else {
				_, _ = fmt.Fprintf(f, "\n%v", v)
			}
			started = true
		}
	}
}

func getMultiResErrorString[T any, TElem Result[T]](
	state State,
	vals []TElem,
) string {
	if len(vals) == 1 {
		return fmt.Sprintf("%s: %v", state.String(), vals[0])
	}

	// find the first error matching state and print it before any other errors.
	fi, fr := getFirstRes(state, vals)

	errb := &strings.Builder{}
	_, _ = fmt.Fprintf(errb, "%s: %v", state.String(), fr)

	// skip unneeded or already printed results...
	//
	// the end result is, when 'state' is Error, it includes only Error.
	// and when 'state' is Panic, it includes both Error and Panic.
	for i, ir := range vals {
		if got := ir.State(); filterResultFunc(got, state) && i != fi {
			_, _ = fmt.Fprintf(errb, "\n%v", ir)
		}
	}
	return errb.String()
}

func unwrapMultiResErrors[T any, TElem Result[T]](
	state State,
	vals []TElem,
	errs []error,
) []error {
	// skip nil or unneeded results...
	//
	// the end result is, when 'state' is Error, it includes only Error.
	// and when 'state' is Panic, it includes both Error and Panic.
	for _, v := range vals {
		switch res := any(v).(type) {
		case IdxRes[T]:
			if res.Result == nil {
				continue
			}
			if got := res.State(); filterResultFunc(got, state) {
				errs = append(errs, IdxError{Idx: res.Idx, Err: res.Err()})
			}
		case GroupRes[T]:
			if res.Result == nil {
				continue
			}
			if got := res.State(); filterResultFunc(got, state) {
				errs = append(errs, GroupError{Err: res.Err()})
			}
		}
	}

	return errs
}

func castMultiResAsError[T any, TElem Result[T]](
	state State,
	vals []TElem,
	target any,
) bool {
	switch perr := target.(type) {
	case *IdxError:
		_, fr := getFirstRes(state, vals)
		if res, ok := any(fr).(IdxRes[T]); ok {
			perr.Idx = res.Idx
			perr.Err = res.Err()
			return true
		}
	case *GroupError:
		_, fr := getFirstRes(state, vals)
		if res, ok := any(fr).(GroupRes[T]); ok {
			perr.Err = res.Err()
			return true
		}
	case *MultiError:
		// include all errors.
		errs := make([]error, 0, len(vals)) // assuming all values are errors.
		perr.Errs = unwrapMultiResErrors(state, vals, errs)
		return true
	}

	// return on non-supported target types.
	return false
}

// getFirstRes returns the first [Result] with [State] state from vals.
func getFirstRes[T any, TElem Result[T]](
	state State,
	vals []TElem,
) (int, TElem) {
	// find the first [Result] with the target [State] state.
	// there must be at least one [Result] value, as this function is called
	// on [Error] or [Panic] results, otherwise the whole result value wouldn't
	// have been created.
	i := slices.IndexFunc(vals, func(ir TElem) bool { return ir.State() == state })
	return i, vals[i]
}
