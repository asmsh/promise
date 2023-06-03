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

// Package result holds types for common and generic result containers,
// and functions to construct them; such containers are needed to return
// arbitrary generic values from functions/methods.
package result

func Val[T any](val T) ValRes[T] {
	return ValRes[T]{val: val}
}

type ValRes[T any] struct {
	val T
}

func (r ValRes[T]) Val() T {
	return r.val
}

func (r ValRes[T]) Err() error {
	return nil
}

func Err[T any](err error) ErrRes[T] {
	return ErrRes[T]{err: err}
}

type ErrRes[T any] struct {
	err error
}

func (r ErrRes[T]) Val() T {
	var v T
	return v
}

func (r ErrRes[T]) Err() error {
	return r.err
}

func ValErr[T any](val T, err error) ValErrRes[T] {
	return ValErrRes[T]{val: val, err: err}
}

type ValErrRes[T any] struct {
	val T
	err error
}

func (r ValErrRes[T]) Val() T {
	return r.val
}

func (r ValErrRes[T]) Err() error {
	return r.err
}
