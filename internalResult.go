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

// fulfilledResultSingleRes is a Result implementation for Fulfilled results
// returned from the Select extension call.
type fulfilledResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r fulfilledResultSingleRes[T, TElem]) Val() TElem   { return r.val }
func (r fulfilledResultSingleRes[T, TElem]) Err() error   { return nil }
func (r fulfilledResultSingleRes[T, TElem]) State() State { return Fulfilled }
func (r fulfilledResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	printSingleRes(r.State().String(), r.val, f, verb)
}

// rejectedResultSingleRes is a Result and error implementation for Rejected
// results returned from the Select extension call.
type rejectedResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r rejectedResultSingleRes[T, TElem]) Val() TElem   { return r.val }
func (r rejectedResultSingleRes[T, TElem]) Err() error   { return r }
func (r rejectedResultSingleRes[T, TElem]) State() State { return Rejected }
func (r rejectedResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	printSingleRes(r.State().String(), r.val, f, verb)
}
func (r rejectedResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%s: %v", r.State().String(), r.val)
}
func (r rejectedResultSingleRes[T, TElem]) Unwrap() error {
	switch res := any(r.val).(type) {
	case IdxRes[T]:
		return IdxError{Idx: res.Idx, Err: res.Err()}
	case GroupRes[T]:
		return GroupError{Err: res.Err()}
	}
	return nil
}

// panickedResultSingleRes is a Result and error implementation for Panicked
// results returned from the Select extension call.
type panickedResultSingleRes[T any, TElem Result[T]] struct {
	val TElem
}

func (r panickedResultSingleRes[T, TElem]) Val() (v TElem) { return v }
func (r panickedResultSingleRes[T, TElem]) Err() error     { return r }
func (r panickedResultSingleRes[T, TElem]) State() State   { return Panicked }
func (r panickedResultSingleRes[T, TElem]) Format(f fmt.State, verb rune) {
	printSingleRes(r.State().String(), r.val, f, verb)
}
func (r panickedResultSingleRes[T, TElem]) Error() string {
	return fmt.Sprintf("%s: %v", r.State().String(), r.val)
}
func (r panickedResultSingleRes[T, TElem]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panickedResultSingleRes[T, TElem]) Unwrap() error {
	switch res := any(r.val).(type) {
	case IdxRes[T]:
		return IdxError{Idx: res.Idx, Err: res.Err()}
	case GroupRes[T]:
		return GroupError{Err: res.Err()}
	}
	return nil
}
func (r panickedResultSingleRes[T, TElem]) As(target any) bool {
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
func (r panickedResultSingleRes[T, TElem]) PanicV() any {
	return getPanicVFromRes(r.val)
}

// fulfilledResultMultiRes is a Result implementation for Fulfilled results
// returned from the All(and AllWait), Any(and AnyWait) or Join extension calls.
type fulfilledResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r fulfilledResultMultiRes[T, TElem]) Val() []TElem { return r.vals }
func (r fulfilledResultMultiRes[T, TElem]) Err() error   { return nil }
func (r fulfilledResultMultiRes[T, TElem]) State() State { return Fulfilled }
func (r fulfilledResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(r.State().String(), r.vals, f, verb)
}

// rejectedResultMultiRes is a Result and error implementation for Rejected
// results returned from the All(and AllWait), Any(and AnyWait) or Join
// extension calls.
type rejectedResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r rejectedResultMultiRes[T, TElem]) Val() []TElem { return r.vals }
func (r rejectedResultMultiRes[T, TElem]) Err() error   { return r }
func (r rejectedResultMultiRes[T, TElem]) State() State { return Rejected }
func (r rejectedResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(r.State().String(), r.vals, f, verb)
}
func (r rejectedResultMultiRes[T, TElem]) Error() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %v", r.State().String(), r.vals[0])
	}

	// find the first Rejected error and print it before any other errors.
	fi, fr := r.firstRejectedRes()

	errb := &strings.Builder{}
	fmt.Fprintf(errb, "%s: %v", r.State().String(), fr)

	for i, ir := range r.vals {
		// not Rejected, or already printed
		if ir.State() != Rejected || i == fi {
			continue
		}
		fmt.Fprintf(errb, "\n%v", ir)
	}
	return errb.String()
}
func (r rejectedResultMultiRes[T, TElem]) Unwrap() []error {
	errs := make([]error, 0, len(r.vals))
	for _, ir := range r.vals {
		if ir.State() != Rejected { // only Rejected is returned
			continue
		}
		switch res := any(ir).(type) {
		case IdxRes[T]:
			errs = append(errs, IdxError{Idx: res.Idx, Err: res.Err()})
		case GroupRes[T]:
			errs = append(errs, GroupError{Err: res.Err()})
		}
	}
	return errs
}
func (r rejectedResultMultiRes[T, TElem]) As(target any) bool {
	switch perr := target.(type) {
	case *IdxError:
		_, fr := r.firstRejectedRes()
		if res, ok := any(fr).(IdxRes[T]); ok {
			perr.Idx = res.Idx
			perr.Err = res.Err()
			return true
		}
	case *GroupError:
		_, fr := r.firstRejectedRes()
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
func (r rejectedResultMultiRes[T, TElem]) firstRejectedRes() (int, TElem) {
	// find the first Rejected Result.
	// there must be at least one Result value, otherwise the whole result value
	// wouldn't have been created, and that value has to be with Rejected State.
	i := slices.IndexFunc(r.vals, func(ir TElem) bool { return ir.State() == Rejected })
	return i, r.vals[i]
}

// panickedResultMultiRes is a Result and error implementation for panic
// results returned from either the All(and AllWait), Any(and AnyWait) or Join
// extension calls.
type panickedResultMultiRes[T any, TElem Result[T]] struct {
	vals []TElem
}

func (r panickedResultMultiRes[T, TElem]) Val() []TElem { return r.vals }
func (r panickedResultMultiRes[T, TElem]) Err() error   { return r }
func (r panickedResultMultiRes[T, TElem]) State() State { return Panicked }
func (r panickedResultMultiRes[T, TElem]) Format(f fmt.State, verb rune) {
	printMultiRes(r.State().String(), r.vals, f, verb)
}
func (r panickedResultMultiRes[T, TElem]) Error() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %v", r.State().String(), r.vals[0])
	}

	// find the first Panicked error and print it before any other errors.
	fi, fr := r.firstPanickedRes()

	errb := &strings.Builder{}
	fmt.Fprintf(errb, "%s: %v", r.State().String(), fr)

	for i, ir := range r.vals {
		// not Rejected nor Panicked, or already printed
		if ir.State() == Fulfilled || i == fi {
			continue
		}
		fmt.Fprintf(errb, "\n%v", ir)
	}
	return errb.String()
}
func (r panickedResultMultiRes[T, TElem]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panickedResultMultiRes[T, TElem]) Unwrap() []error {
	errs := make([]error, 0, len(r.vals))
	for _, ir := range r.vals {
		if ir.State() == Fulfilled { // only Panicked and Rejected are returned
			continue
		}
		switch res := any(ir).(type) {
		case IdxRes[T]:
			errs = append(errs, IdxError{Idx: res.Idx, Err: res.Err()})
		case GroupRes[T]:
			errs = append(errs, GroupError{Err: res.Err()})
		}
	}
	return errs
}
func (r panickedResultMultiRes[T, TElem]) As(target any) bool {
	switch perr := target.(type) {
	case *PanicError:
		perr.V = r.PanicV()
	case *IdxError:
		_, fr := r.firstPanickedRes()
		if res, ok := any(fr).(IdxRes[T]); ok {
			perr.Idx = res.Idx
			perr.Err = res.Err()
			return true
		}
	case *GroupError:
		_, fr := r.firstPanickedRes()
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
func (r panickedResultMultiRes[T, TElem]) PanicV() any {
	_, fr := r.firstPanickedRes()
	return getPanicVFromRes(fr)
}
func (r panickedResultMultiRes[T, TElem]) firstPanickedRes() (int, TElem) {
	// find the first Panicked Result.
	// there must be at least one Result value, otherwise the whole result value
	// wouldn't have been created, and that value has to be with Panicked State.
	i := slices.IndexFunc(r.vals, func(ir TElem) bool { return ir.State() == Panicked })
	return i, r.vals[i]
}

// errPromiseConsumedResult is a static error result that returns ErrPromiseConsumed.
// it's used instead of saving the ErrPromiseConsumed error in a generic errResult value.
type errPromiseConsumedResult[T any] struct{}

func (r errPromiseConsumedResult[T]) Val() (v T)   { return v }
func (r errPromiseConsumedResult[T]) Err() error   { return ErrPromiseConsumed }
func (r errPromiseConsumedResult[T]) State() State { return Rejected }
func (r errPromiseConsumedResult[T]) String() string {
	return fmt.Sprintf("rejected: %s", ErrPromiseConsumed.Error())
}

// errPromiseCtxNilDoneResult is a static error result that returns ErrPromiseNilCtxDone.
// it's used instead of saving the ErrPromiseNilCtxDone error in a generic errResult value.
type errPromiseCtxNilDoneResult[T any] struct{}

func (r errPromiseCtxNilDoneResult[T]) Val() (v T)   { return v }
func (r errPromiseCtxNilDoneResult[T]) Err() error   { return ErrPromiseNilCtxDone }
func (r errPromiseCtxNilDoneResult[T]) State() State { return Rejected }
func (r errPromiseCtxNilDoneResult[T]) String() string {
	return fmt.Sprintf("rejected: %s", ErrPromiseNilCtxDone.Error())
}

// errPromiseGroupDoneResult is a static error result that returns ErrPromiseGroupDone.
// it's used instead of saving the ErrPromiseGroupDone error in a generic errResult value.
type errPromiseGroupDoneResult[T any] struct{}

func (r errPromiseGroupDoneResult[T]) Val() (v T)   { return v }
func (r errPromiseGroupDoneResult[T]) Err() error   { return ErrPromiseGroupDone }
func (r errPromiseGroupDoneResult[T]) State() State { return Rejected }
func (r errPromiseGroupDoneResult[T]) String() string {
	return fmt.Sprintf("rejected: %s", ErrPromiseGroupDone.Error())
}

// errPromiseGroupBusyResult is a static error result that returns ErrPromiseGroupBusy.
// it's used instead of saving the ErrPromiseGroupBusy error in a generic errResult value.
type errPromiseGroupBusyResult[T any] struct{}

func (r errPromiseGroupBusyResult[T]) Val() (v T)   { return v }
func (r errPromiseGroupBusyResult[T]) Err() error   { return ErrPromiseGroupBusy }
func (r errPromiseGroupBusyResult[T]) State() State { return Rejected }
func (r errPromiseGroupBusyResult[T]) String() string {
	return fmt.Sprintf("rejected: %s", ErrPromiseGroupBusy.Error())
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
