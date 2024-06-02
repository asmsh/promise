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

// IdxRes is a positional result view, that represents the result of the promise
// at index idx in the original list provided.
type IdxRes[T any] struct {
	Idx int
	Result[T]
}

func (ir IdxRes[T]) String() string {
	if ir.Result == nil {
		return "<nil>"
	}
	return fmt.Sprintf("[%d]%v", ir.Idx, ir.Result)
}

// internal result types
// the purpose of these types is to reduce allocations when setting the Result of
// the resolved promise, and implement the required logic to investigate the error
// structure, using the error, errors.Unwrap, errors.Is and errors.As interfaces.
// also to ensure consistent string conversion of the results.

// fulfilledResultSingleIdxRes is a Result implementation for Fulfilled results
// returned from the Select extension call.
type fulfilledResultSingleIdxRes[T any] struct {
	val IdxRes[T]
}

func (r fulfilledResultSingleIdxRes[T]) Val() IdxRes[T] { return r.val }
func (r fulfilledResultSingleIdxRes[T]) Err() error     { return nil }
func (r fulfilledResultSingleIdxRes[T]) State() State   { return Fulfilled }
func (r fulfilledResultSingleIdxRes[T]) String() string {
	return fmt.Sprintf("%s: %s", r.State().String(), r.val.String())
}

// rejectedResultSingleIdxRes is a Result and error implementation for Rejected
// results returned from the Select extension call.
type rejectedResultSingleIdxRes[T any] struct {
	val IdxRes[T]
}

func (r rejectedResultSingleIdxRes[T]) Val() IdxRes[T] { return r.val }
func (r rejectedResultSingleIdxRes[T]) Err() error     { return r }
func (r rejectedResultSingleIdxRes[T]) State() State   { return Rejected }
func (r rejectedResultSingleIdxRes[T]) String() string {
	return fmt.Sprintf("%s: %s", r.State().String(), r.val.String())
}
func (r rejectedResultSingleIdxRes[T]) Error() string {
	return fmt.Sprintf("%s: %s", r.State().String(), r.val.String())
}
func (r rejectedResultSingleIdxRes[T]) Unwrap() error {
	return IdxError{Idx: r.val.Idx, Err: r.val.Err()}
}

// panickedResultSingleIdxRes is a Result and error implementation for Panicked
// results returned from the Select extension call.
type panickedResultSingleIdxRes[T any] struct {
	val IdxRes[T]
}

func (r panickedResultSingleIdxRes[T]) Val() (v IdxRes[T]) { return v }
func (r panickedResultSingleIdxRes[T]) Err() error         { return r }
func (r panickedResultSingleIdxRes[T]) State() State       { return Panicked }
func (r panickedResultSingleIdxRes[T]) String() string {
	return fmt.Sprintf("%s: %s", r.State().String(), r.val.String())
}
func (r panickedResultSingleIdxRes[T]) Error() string {
	return fmt.Sprintf("%s: %s", r.State().String(), r.val.String())
}
func (r panickedResultSingleIdxRes[T]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panickedResultSingleIdxRes[T]) Unwrap() error {
	return IdxError{Idx: r.val.Idx, Err: r.val.Err()}
}
func (r panickedResultSingleIdxRes[T]) As(target any) bool {
	switch perr := target.(type) {
	default:
		// return on non-supported target types.
		return false
	case *PanicError:
		perr.V = getPanicVFromRes(r.val.Result)
	case *IdxError:
		perr.Idx = r.val.Idx
		perr.Err = r.val.Err()
	}
	return true
}

// fulfilledResultMultiIdxRes is a Result implementation for Fulfilled results
// returned from the All(and AllWait), Any(and AnyWait) or Join extension calls.
type fulfilledResultMultiIdxRes[T any] struct {
	vals []IdxRes[T]
}

func (r fulfilledResultMultiIdxRes[T]) Val() []IdxRes[T] { return r.vals }
func (r fulfilledResultMultiIdxRes[T]) Err() error       { return nil }
func (r fulfilledResultMultiIdxRes[T]) State() State     { return Fulfilled }
func (r fulfilledResultMultiIdxRes[T]) String() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %s", r.State().String(), r.vals[0].String())
	}

	errb := strings.Builder{}
	for _, ir := range r.vals {
		if errb.Len() == 0 {
			errb.WriteString(r.State().String())
			errb.WriteString(": ")
		} else {
			errb.WriteByte('\n')
		}
		errb.WriteString(ir.String())
	}
	return errb.String()
}

// rejectedResultMultiIdxRes is a Result and error implementation for Rejected
// results returned from the All(and AllWait), Any(and AnyWait) or Join
// extension calls.
type rejectedResultMultiIdxRes[T any] struct {
	vals []IdxRes[T]
}

func (r rejectedResultMultiIdxRes[T]) Val() []IdxRes[T] { return r.vals }
func (r rejectedResultMultiIdxRes[T]) Err() error       { return r }
func (r rejectedResultMultiIdxRes[T]) State() State     { return Rejected }
func (r rejectedResultMultiIdxRes[T]) String() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %s", r.State().String(), r.vals[0].String())
	}

	errb := strings.Builder{}
	for _, ir := range r.vals {
		if errb.Len() == 0 {
			errb.WriteString(r.State().String())
			errb.WriteString(": ")
		} else {
			errb.WriteByte('\n')
		}
		errb.WriteString(ir.String())
	}
	return errb.String()
}
func (r rejectedResultMultiIdxRes[T]) Error() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %s", r.State().String(), r.vals[0].String())
	}

	// find the first Rejected error and print it before any other errors.
	fi := slices.IndexFunc(r.vals, func(ir IdxRes[T]) bool { return ir.State() == Rejected })
	errb := strings.Builder{}
	errb.WriteString(r.State().String())
	errb.WriteString(": ")
	errb.WriteString(r.vals[fi].String())
	for i, ir := range r.vals {
		if ir.State() != Rejected || i == fi { // not Rejected, or already printed
			continue
		}
		errb.WriteByte('\n')
		errb.WriteString(ir.String())
	}
	return errb.String()
}
func (r rejectedResultMultiIdxRes[T]) Unwrap() []error {
	if len(r.vals) == 1 {
		return []error{IdxError{Idx: r.vals[0].Idx, Err: r.vals[0].Err()}}
	}

	// multiple results, return errors only.
	errs := make([]error, 0, len(r.vals))
	for _, ir := range r.vals {
		if ir.State() != Rejected { // only Rejected is returned
			continue
		}
		errs = append(errs, IdxError{Idx: ir.Idx, Err: ir.Err()})
	}
	return errs
}
func (r rejectedResultMultiIdxRes[T]) As(target any) bool {
	switch perr := target.(type) {
	default:
		// return on non-supported target types.
		return false
	case *IdxError:
		// find the first panic error and save it in the target.
		i := slices.IndexFunc(r.vals, func(ir IdxRes[T]) bool { return ir.State() == Rejected })
		perr.Idx = r.vals[i].Idx
		perr.Err = r.vals[i].Err()
	case *MultiIdxError:
		// include all errors.
		perr.errs = r.Unwrap()
	}
	return true
}

// panickedResultMultiIdxRes is a Result and error implementation for panic
// results returned from either the All(and AllWait), Any(and AnyWait) or Join
// extension calls.
type panickedResultMultiIdxRes[T any] struct {
	vals []IdxRes[T]
}

func (r panickedResultMultiIdxRes[T]) Val() []IdxRes[T] { return nil }
func (r panickedResultMultiIdxRes[T]) Err() error       { return r }
func (r panickedResultMultiIdxRes[T]) State() State     { return Panicked }
func (r panickedResultMultiIdxRes[T]) String() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %s", r.State().String(), r.vals[0].String())
	}

	errb := strings.Builder{}
	for _, ir := range r.vals {
		if errb.Len() == 0 {
			errb.WriteString(r.State().String())
			errb.WriteString(": ")
		} else {
			errb.WriteByte('\n')
		}
		errb.WriteString(ir.String())
	}
	return errb.String()
}
func (r panickedResultMultiIdxRes[T]) Error() string {
	if len(r.vals) == 1 {
		return fmt.Sprintf("%s: %s", r.State().String(), r.vals[0].String())
	}

	// find the first Panicked error and print it before any other errors.
	fi := slices.IndexFunc(r.vals, func(ir IdxRes[T]) bool { return ir.State() == Panicked })
	errb := strings.Builder{}
	errb.WriteString(r.State().String())
	errb.WriteString(": ")
	errb.WriteString(r.vals[fi].String())
	for i, ir := range r.vals {
		if ir.State() == Fulfilled || i == fi { // not Rejected nor Panicked, or already printed
			continue
		}
		errb.WriteByte('\n')
		errb.WriteString(ir.String())
	}
	return errb.String()
}
func (r panickedResultMultiIdxRes[T]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panickedResultMultiIdxRes[T]) Unwrap() []error {
	if len(r.vals) == 1 {
		return []error{IdxError{Idx: r.vals[0].Idx, Err: r.vals[0].Err()}}
	}

	// multiple results, return errors only.
	errs := make([]error, 0, len(r.vals))
	for _, ir := range r.vals {
		if ir.State() == Fulfilled { // only Panicked and Rejected are returned
			continue
		}
		errs = append(errs, IdxError{Idx: ir.Idx, Err: ir.Err()})
	}
	return errs
}
func (r panickedResultMultiIdxRes[T]) As(target any) bool {
	switch perr := target.(type) {
	default:
		// return on non-supported target types.
		return false
	case *PanicError:
		// find the first panic error and save it in the target.
		i := slices.IndexFunc(r.vals, func(ir IdxRes[T]) bool { return ir.State() == Panicked })
		perr.V = getPanicVFromRes(r.vals[i].Result)
	case *IdxError:
		// find the first panic error and save it in the target.
		i := slices.IndexFunc(r.vals, func(ir IdxRes[T]) bool { return ir.State() == Panicked })
		perr.Idx = r.vals[i].Idx
		perr.Err = r.vals[i].Err()
	case *MultiIdxError:
		// include all errors.
		perr.errs = r.Unwrap()
	}
	return true
}
