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

// Result is a Container for generic result values
type Result[T any] interface {
	Val() T
	Err() error
}

func Empty[T any]() EmptyResult[T] {
	return EmptyResult[T]{}
}

func Val[T any](val T) ValResult[T] {
	return ValResult[T]{val: val}
}

func Err[T any](err error) ErrResult[T] {
	return ErrResult[T]{err: err}
}

func ValErr[T any](val T, err error) ValErrResult[T] {
	return ValErrResult[T]{val: val, err: err}
}

type EmptyResult[T any] struct{}
type ValResult[T any] struct{ val T }
type ErrResult[T any] struct{ err error }
type ValErrResult[T any] struct {
	val T
	err error
}

func (r EmptyResult[T]) Val() (v T) { return v }
func (r ValResult[T]) Val() T       { return r.val }
func (r ErrResult[T]) Val() (v T)   { return v }
func (r ValErrResult[T]) Val() T    { return r.val }

func (r EmptyResult[T]) Err() error  { return nil }
func (r ValResult[T]) Err() error    { return nil }
func (r ErrResult[T]) Err() error    { return r.err }
func (r ValErrResult[T]) Err() error { return r.err }
