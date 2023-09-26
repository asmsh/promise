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

func (r emptyResult[T]) Val() (v T)  { return v }
func (r valResult[T]) Val() (v T)    { return r.val }
func (r errResult[T]) Val() (v T)    { return v }
func (r valErrResult[T]) Val() (v T) { return r.val }

func (r emptyResult[T]) Err() error  { return nil }
func (r valResult[T]) Err() error    { return nil }
func (r errResult[T]) Err() error    { return r.err }
func (r valErrResult[T]) Err() error { return r.err }
