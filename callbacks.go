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

import "github.com/asmsh/promise/result"

type callbackFunc[T any] interface {
	call(res Result[T], s uint32) Result[T]
}

type thenCallback[T any] func(val T) Result[T]
type catchCallback[T any] func(val T, err error) Result[T]
type recoverCallback[T any] func(v any) Result[T]
type finallyCallback[T any] func(s Status) Result[T]

func (cb thenCallback[T]) call(res Result[T], s uint32) Result[T] {
	return cb(res.Val())
}
func (cb catchCallback[T]) call(res Result[T], s uint32) Result[T] {
	return cb(res.Val(), res.Err())
}
func (cb recoverCallback[T]) call(res Result[T], s uint32) Result[T] {
	return cb(res.Err().(*UncaughtPanic).v)
}
func (cb finallyCallback[T]) call(res Result[T], s uint32) Result[T] {
	return cb(Status(s))
}

func runCallback[T any](
	p *GenericPromise[T],
	prevRes Result[T],
	prevStatus uint32,
	cb callbackFunc[T],
) {
	// defer the return handler to handle panics and runtime.Goexit calls
	resP := new(Result[T])
	defer p.handleReturns(resP)

	// run the callback and extract the result
	res := cb.call(prevRes, prevStatus)

	// if the callback returned invalid result, set the promise result to
	// the appropriate error result, otherwise set it to the value returned.
	if res == nil {
		*resP = result.Err[T](ErrPromiseNilResult)
	} else {
		*resP = res
	}
}