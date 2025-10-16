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
	return emptyResult[T]{}
}

func ValRes[T any](val T) Result[T] {
	return valResult[T]{val: val}
}

func ErrRes[T any](err error) Result[T] {
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
	if pr, ok := res.(resultPanicV); ok {
		return pr.PanicV()
	}
	return PanicError{V: res}
}

// IdxRes is a positional result view, that represents the result of the promise
// at index Idx in the original list provided.
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

// GroupRes
// TODO: choose another name (??)
// the need of this type is to add fields later that can identify the promise
// that generated that group's result.
type GroupRes[T any] struct {
	// possible fields:
	// - InitType(constructor type): enum(Go, Ctx, GoCtxRes, GoErr)
	Result[T]
}

func (gr GroupRes[T]) String() string {
	if gr.Result == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", gr.Result)
}

// UnwrapMultiRes returns the values wrapped inside a multi result value.
//
// Examples:
//
// Working with plain [Promise] values.
//
//	p := All[any]( /* list of promises */ )
//	multiRes := p.Res()
//	vals := UnwrapMultiRes(multiRes)
//	// do something with vals...
//
// Working with a [Group] value.
//
//	var pg Group[any]
//	/* start some promises on pg */
//	multiRes := pg.AllWaitRes()
//	vals := UnwrapMultiRes(multiRes)
//	// do something with vals...
func UnwrapMultiRes[T any, TRes Result[T], TMRes Result[[]TRes]](
	multiRes TMRes,
) []T {
	multiResVal := multiRes.Val()
	res := make([]T, 0, len(multiResVal))
	for _, r := range multiResVal {
		res = append(res, r.Val())
	}
	return res
}

type emptyResult[T any] struct{}

func (r emptyResult[T]) Val() (v T)   { return v }
func (r emptyResult[T]) Err() error   { return nil }
func (r emptyResult[T]) State() State { return Success }
func (r emptyResult[T]) String() string {
	return "Success: <nil>"
}

type valResult[T any] struct{ val T }

func (r valResult[T]) Val() (v T)   { return r.val }
func (r valResult[T]) Err() error   { return nil }
func (r valResult[T]) State() State { return Success }
func (r valResult[T]) String() string {
	return fmt.Sprintf("Success: %v", r.val)
}

type errResult[T any] struct{ err error }

func (r errResult[T]) Val() (v T)   { return v }
func (r errResult[T]) Err() error   { return r.err }
func (r errResult[T]) State() State { return Error }
func (r errResult[T]) String() string {
	return fmt.Sprintf("Error: %s", r.err.Error())
}

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

type result[T any] struct {
	val   T
	err   error
	state State
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

type panicResult[T any] struct{ v any }

func (r panicResult[T]) Val() (v T)   { return v }
func (r panicResult[T]) Err() error   { return r }
func (r panicResult[T]) State() State { return Panic }
func (r panicResult[T]) String() string {
	// same error message and format as the PanicError
	return fmt.Sprintf("Panic: %v", r.v)
}

// additional methods for the panicResult type, to make it act like a PanicError.
func (r panicResult[T]) Error() string { return r.String() }
func (r panicResult[T]) Is(target error) bool {
	// make this error result implement the identity panic error value.
	return target == ErrPromisePanicked
}
func (r panicResult[T]) Unwrap() error {
	// try to return the panic value as an error value if it's really an error value.
	if err, ok := r.v.(error); ok {
		return err
	}
	return nil
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
