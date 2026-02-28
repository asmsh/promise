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

func EmptyRes[T any]() Result[T] {
	return zeroResult[T]{}
}

func ValRes[T any](val T) Result[T] {
	return valResult[T]{val: val}
}

func ErrRes[T any](err error) Result[T] {
	if err == nil {
		return zeroResult[T]{}
	}
	return errResult[T]{err: err}
}

func ValErrRes[T any](val T, err error) Result[T] {
	return valErrResult[T]{val: val, err: err}
}

func PanicRes[T any](v any) Result[T] {
	return panicResult[T]{v: v}
}

// resultPanicV must be implemented for any custom (from outside this package)
// [Result] type that returns the [Panic] [State].
// it's required for retrieving the panic value when calling the [GroupConfig.UnhandledPanicCB],
// otherwise, a new [PanicError] is gonna be passed wrapping the original [Result]
// value as its V field ([PanicError.V]).
type resultPanicV interface {
	PanicV() any
}

func getPanicVFromRes[T any](res Result[T]) any {
	if pr, ok := res.Err().(resultPanicV); ok {
		return pr.PanicV()
	}
	return PanicError{V: res}
}

// IdxRes wraps a [Result] value for the [Promise] at index Idx in the list
// of values passed to any of the extensions functions ([Any], [All], etc.).
//
// The zero value is useless and will panic on any [Result] method call.
type IdxRes[T any] struct {
	Idx int
	Result[T]
}

func (r IdxRes[T]) String() string {
	if r.Result == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", r.Result)
}

// GroupRes wraps a [Result] value that's returned from any of the [Group]'s
// Res methods ([Group.AllRes], [Group.AnyWaitRes], etc.).
//
// The zero value is useless and will panic on any [Result] method call.
type GroupRes[T any] struct {
	Result[T]
}

func (r GroupRes[T]) String() string {
	if r.Result == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", r.Result)
}

// MultiRes creates a [Result] value of the provided multiRes as its [Result.Val],
// with its [Result.State] being the provided state.
//
// It's useful when returning a filtered multi result value, that's a [Result]
// which its [Result.Val] returns a slice of [Result] values.
//
// For example:
//
//	p := All[any]( /* list of promises that possibly panicked */ )
//
//	p.Recover(func(ctx context.Context, multiRes Result[[]IdxRes[any]]) Result[[]IdxRes[any]] {
//		// clone the received [Result] value before filtering it.
//		// note: this is needed as the returned slice (from [Result.Val])
//		// is the same value returned from all [Result.Val] calls.
//		newMultiResVal := make([]IdxRes[any], len(multiRes.Val()))
//		copy(newMultiResVal, multiRes.Val())
//
//		// filter the new [Result] values, to remove the handled values.
//		newMultiResVal = slices.DeleteFunc(newMultiResVal, func(i IdxRes[any]) bool {
//			return i.State() == Panic
//		})
//
//		// create a new [Result] with the needed [State] and [Result] values.
//		newMultiRes := MultiRes(Success, newMultiResVal...)
//
//		// return the filtered and handled [Result].
//		return newMultiRes
//	})
//
// Note: the returned value's [Result.Val] slice shouldn't be modified,
// and any modification should be made on a cloned value.
func MultiRes[TElem Result[T], T any](
	state State,
	multiRes ...TElem,
) Result[[]TElem] {
	switch state {
	case Success:
		return successResultMultiRes[T, TElem]{multiRes}
	case Error:
		return errorResultMultiRes[T, TElem]{multiRes}
	case Panic:
		return panicResultMultiRes[T, TElem]{multiRes}
	default:
		panic("promise: unexpected Result state: " + state.String())
	}
}

// UnwrapMultiResVal returns the values wrapped inside a multi result value,
// that's a [Result] which its [Result.Val] returns a slice of [Result] values.
// It returns the result of calling the [Result.Val] method on each [Result]
// value wrapped in the provided multiRes.
//
// Examples:
//
// Working with plain [Promise] values.
//
//	p := All[any]( /* list of promises */ )
//	multiRes := p.Res()
//	vals := UnwrapMultiResVal(multiRes)
//	// do something with vals...
//
// Working with a [Group] value.
//
//	var pg Group[any]
//	/* start some promises on pg */
//	multiRes := pg.AllWaitRes()
//	vals := UnwrapMultiResVal(multiRes)
//	// do something with vals...
//
// It preserves empty values (like nil), which might be returned from
// [Result.Val] because of a [Success], [Error], or [Panic] [Result]
// with empty [Result.Val], so a subsequent filtering might be needed.
//
// For example:
//
//	/* multiRes contains Success, Error, or Panic with nil Result.Val */
//	vals := UnwrapMultiResVal(multiRes)
//	vals = slices.DeleteFunc(vals, func(v any) bool {
//		// filtering logic based on v's value...
//		return v == nil
//	})
func UnwrapMultiResVal[T any, TElem Result[T], TRes Result[[]TElem]](
	multiRes TRes,
) []T {
	multiResVal := multiRes.Val()
	res := make([]T, 0, len(multiResVal))
	for _, r := range multiResVal {
		res = append(res, r.Val())
	}
	return res
}

// implementations of the above [Result] values...

// zeroResult is a [Result] implementation for cases where
// the [State] is [Success] and there's no value nor error.
// it's created as an efficient way to represent such results
// without using a type that makes space for an always-empty
// value and error.
//
// it's the type used whenever a callback returns `nil` as
// the [Result] value.
type zeroResult[T any] struct{}

func (r zeroResult[T]) Val() (v T)   { return v }
func (r zeroResult[T]) Err() error   { return nil }
func (r zeroResult[T]) State() State { return Success }
func (r zeroResult[T]) String() string {
	return "Success: <nil>"
}

// valResult is a [Result] implementation for cases where
// the [State] is [Success] and there's only value (no error).
// it's created as an efficient way to represent such results
// without using a type that makes space for an always-nil error.
type valResult[T any] struct{ val T }

func (r valResult[T]) Val() (v T)   { return r.val }
func (r valResult[T]) Err() error   { return nil }
func (r valResult[T]) State() State { return Success }
func (r valResult[T]) String() string {
	return fmt.Sprintf("Success: %v", r.val)
}

// errResult is a [Result] implementation for cases where
// the [State] is [Error] and there's only error (no value).
// it's created as an efficient way to represent such results
// without using a type that makes space for an always-nil error.
type errResult[T any] struct{ err error }

func (r errResult[T]) Val() (v T)   { return v }
func (r errResult[T]) Err() error   { return r.err }
func (r errResult[T]) State() State { return Error }
func (r errResult[T]) String() string {
	return fmt.Sprintf("Error: %s", r.err.Error())
}

// valErrResult is a [Result] implementation for cases where
// the [State] is either [Success] or [Error], based on whether
// the included error is nil or not, respectively.
// it's created as an efficient way to represent such results
// without using a type that saves the [State] value.
type valErrResult[T any] struct {
	val T
	err error
}

func (r valErrResult[T]) Val() (v T) { return r.val }
func (r valErrResult[T]) Err() error { return r.err }
func (r valErrResult[T]) State() State {
	if r.err == nil {
		return Success
	}
	return Error
}
func (r valErrResult[T]) String() string {
	if r.err == nil {
		return fmt.Sprintf("Success: %v", r.val)
	}
	return fmt.Sprintf("Error: (%v, %s)", r.val, r.err.Error())
}

// panicResult is a [Result] implementation for cases where
// the [State] is [Panic].
// it's created as an efficient way to represent such results
// without using a type that makes space for unnecessarily fields.
//
// it's also an [error] implementation for the error returned from
// the [Result.Err] method.
// it implements the [errors.As] and [errors.Is] methods to make
// its values work as a [PanicError] value.
//
// it also implements a PanicV() method, for returning the underlying
// panic value, which is used when returning the panic value, similar
// to a [PanicError] value.
type panicResult[T any] struct{ v any }

func (r panicResult[T]) Val() (v T)   { return v }
func (r panicResult[T]) Err() error   { return r }
func (r panicResult[T]) State() State { return Panic }
func (r panicResult[T]) Error() string {
	// same error message and format as the PanicError
	return fmt.Sprintf("Panic: %v", r.v)
}
func (r panicResult[T]) Unwrap() error {
	// try to return the panic value as an error value if it's really an error value.
	if err, ok := r.v.(error); ok {
		return err
	}
	return nil
}
func (r panicResult[T]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panicResult[T]) As(target any) bool {
	// populate the expected target with panic value.
	if perr, ok := target.(*PanicError); ok {
		perr.V = r.v
		return true
	}
	return false
}
func (r panicResult[T]) PanicV() any { return r.v }

// result is a [Result] implementation that saves all the elements explicitly.
type result[T any] struct {
	val   T
	err   error
	state State
}

func newResult[T any](val T, err error, state State) Result[T] {
	return result[T]{val: val, err: err, state: state}
}

func (r result[T]) Val() (v T)   { return r.val }
func (r result[T]) Err() error   { return r.err }
func (r result[T]) State() State { return r.state }
func (r result[T]) String() string {
	if r.state == Success {
		return fmt.Sprintf("Success: %v", r.val)
	} else if r.state == Error {
		return fmt.Sprintf("Error: (%v, %s)", r.val, r.err.Error())
	} else {
		return fmt.Sprintf("Panic: %s", r.err.Error())
	}
}

// ctxResult is a [Result] implementation that's used with the [Ctx]
// constructors to save the passed [context.Context] value.
type ctxResult[T any] struct{ ctx context.Context }

func (r ctxResult[T]) Val() (v T) { return v }
func (r ctxResult[T]) Err() error { return r.ctx.Err() }
func (r ctxResult[T]) State() State {
	if r.ctx.Err() != nil {
		return Error
	}
	return Success
}
func (r ctxResult[T]) String() string {
	if err := r.ctx.Err(); err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	}
	return "Success: <nil>"
}
func (r ctxResult[T]) WaitChan() <-chan struct{} { return r.ctx.Done() }
