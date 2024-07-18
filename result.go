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

func (r emptyResult[T]) String() string {
	return "fulfilled: <nil>"
}
func (r valResult[T]) String() string {
	return fmt.Sprintf("fulfilled: %v", r.val)
}
func (r errResult[T]) String() string {
	return fmt.Sprintf("rejected: %s", r.err.Error())
}
func (r valErrResult[T]) String() string {
	return fmt.Sprintf("rejected: (%v, %s)", r.val, r.err.Error())
}
func (r ctxResult[T]) String() string {
	if err := r.ctx.Err(); err != nil {
		return fmt.Sprintf("rejected: %s", err.Error())
	}
	return "fulfilled: <nil>"
}
func (r result[T]) String() string {
	if r.state == Fulfilled {
		return fmt.Sprintf("fulfilled: %v", r.val)
	} else if r.state == Rejected {
		return fmt.Sprintf("rejected: (%v, %s)", r.val, r.err.Error())
	} else {
		return fmt.Sprintf("panicked: %s", r.err.Error())
	}
}
