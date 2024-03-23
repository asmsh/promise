// Copyright 2023 Ahmad Sameh(asmsh)
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
	"context"
	"fmt"
)

// Result is a Container for generic result values
type Result[T any] interface {
	Val() T
	Err() error
	State() State
}

func Empty[T any]() Result[T] {
	return emptyResult[T]{}
}

func Val[T any](val T) Result[T] {
	return valResult[T]{val: val}
}

func Err[T any](err error) Result[T] {
	return errResult[T]{err: err}
}

func ValErr[T any](val T, err error) Result[T] {
	return valErrResult[T]{val: val, err: err}
}

type emptyResult[T any] struct{}
type valResult[T any] struct{ val T }
type errResult[T any] struct{ err error }
type valErrResult[T any] struct {
	val T
	err error
}
type result[T any] struct {
	val   T
	err   error
	state State
}
type ctxResult[T any] struct{ ctx context.Context }

func (r emptyResult[T]) Val() (v T)  { return v }
func (r valResult[T]) Val() (v T)    { return r.val }
func (r errResult[T]) Val() (v T)    { return v }
func (r valErrResult[T]) Val() (v T) { return r.val }
func (r ctxResult[T]) Val() (v T)    { return v }
func (r result[T]) Val() (v T)       { return r.val }

func (r emptyResult[T]) Err() error  { return nil }
func (r valResult[T]) Err() error    { return nil }
func (r errResult[T]) Err() error    { return r.err }
func (r valErrResult[T]) Err() error { return r.err }
func (r ctxResult[T]) Err() error    { return r.ctx.Err() }
func (r result[T]) Err() error       { return r.err }

func (r emptyResult[T]) State() State  { return Fulfilled }
func (r valResult[T]) State() State    { return Fulfilled }
func (r errResult[T]) State() State    { return Rejected }
func (r valErrResult[T]) State() State { return Rejected }
func (r ctxResult[T]) State() State {
	if r.ctx.Err() != nil {
		return Rejected
	}
	return Fulfilled
}
func (r result[T]) State() State { return r.state }

func (r emptyResult[T]) String() string  { return "[fulfilled] nil" }
func (r valResult[T]) String() string    { return fmt.Sprintf("[fulfilled] %v", r.val) }
func (r errResult[T]) String() string    { return fmt.Sprintf("[rejected] (nil, %s)", r.err) }
func (r valErrResult[T]) String() string { return fmt.Sprintf("[rejected] (%v, %s)", r.val, r.err) }
func (r ctxResult[T]) String() string {
	if err := r.ctx.Err(); err != nil {
		return fmt.Sprintf("[rejected] (nil, %s)", err)
	}
	return "[fulfilled] nil"
}
func (r result[T]) String() string {
	if r.state == Fulfilled {
		return fmt.Sprintf("[fulfilled] %v", r.val)
	} else if r.state == Rejected {
		return fmt.Sprintf("[rejected] (nil, %s)", r.err)
	} else {
		return fmt.Sprintf("[panicked] %s", r.err)
	}
}

// common error results

// errPromisePanickedResult is used to wrap the panic value internally.
// It's used to allow the res value of the promise to use the Result type,
// regardless of the promise resolved state.
// It's used to wrap the panic value on individual promises, in comparison with
// the errPromisePanickedIdxResult type, which is used to wrap join promises.
type errPromisePanickedResult[T any] struct{ v any }

func (r errPromisePanickedResult[T]) Val() (v T)   { return v }
func (r errPromisePanickedResult[T]) Err() error   { return UncaughtPanic{v: r.v} }
func (r errPromisePanickedResult[T]) State() State { return Panicked }

// errPromiseConsumedResult is a static error result that returns ErrPromiseConsumed.
// it's used instead of saving the ErrPromiseConsumed error in a generic errResult value.
type errPromiseConsumedResult[T any] struct{}

func (r errPromiseConsumedResult[T]) Val() (v T)   { return v }
func (r errPromiseConsumedResult[T]) Err() error   { return ErrPromiseConsumed }
func (r errPromiseConsumedResult[T]) State() State { return Rejected }

// extension result types...
// the following types are private, as they are used only internally.

// errPromisePanickedIdxResult is the error value type of the promise returned from
// either the All or the Any extension calls, when it's resolved to Panicked.
// The value is passed to the Catch callback or returned from the Res method.
//
// The purpose of this type is to reduce allocations when setting the Result of
// the resolved promise, and implement the required logic to investigate the error
// structure, using the errors.As and errors.Is functions.
//
// We are using a new type instead of reusing the errPromisePanickedResult type,
// because ???
// TODO: find a reasonable need for this type, otherwise the errPromisePanickedResult can be used.
type errPromisePanickedIdxResult[T any] struct {
	vals []IdxRes[T]
}

func (r errPromisePanickedIdxResult[T]) Val() []IdxRes[T] {
	// panics don't produce values. keep it like that.
	return nil
}
func (r errPromisePanickedIdxResult[T]) Err() error   { return r }
func (r errPromisePanickedIdxResult[T]) State() State { return Panicked }

// TODO: implement support for errors.As & errors.Is
func (r errPromisePanickedIdxResult[T]) Error() string {
	if len(r.vals) == 1 {
		return r.vals[0].Err().Error()
	}
	return fmt.Sprint(r.vals)
}

// errPromiseRejectedIdxResult is the error value type of the promise returned from
// either the All or the Any extension calls, when it's resolved to Rejected.
// The value is passed to the Catch callback or returned from the Res method.
//
// The purpose of this type is to reduce allocations when setting the Result of
// the resolved promise, and implement the required logic to investigate the error
// structure, using the errors.As and errors.Is functions.
type errPromiseRejectedIdxResult[T any] struct {
	vals []IdxRes[T]
}

func (r errPromiseRejectedIdxResult[T]) Val() []IdxRes[T] {
	return r.vals
}
func (r errPromiseRejectedIdxResult[T]) Err() error {
	return r
}
func (r errPromiseRejectedIdxResult[T]) State() State { return Rejected }

// TODO: implement support for errors.As & errors.Is
func (r errPromiseRejectedIdxResult[T]) Error() string {
	if len(r.vals) == 1 {
		return r.vals[0].Err().Error()
	}
	return fmt.Sprint(r.vals)
}
