// Copyright 2020 Ahmad Sameh(asmsh)
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

package main

import (
	"sync"

	"github.com/asmsh/promise"
)

// The following main function shows how a Promise maybe used to recover
// from panics that happen in goroutines created by a 'go' call, in a much
// simpler way than how it can be done using the standard way(library).
func main() {
	p := promise.Go(func() {
		/* do some work, asynchronously */
		aFuncThatMayPanic()
	}).Recover(func(v interface{}, ok bool) (res promise.Res) {
		// handle the panic..
		return
	})

	/* do some other work */

	p.Wait() // wait for the async work to finish

	/* do some other work */
}

// just any function that may panic
func aFuncThatMayPanic() {}

// The following function is equivalent to the above one, but using
// the standard library and the language's primitives.

func stdAlt() {
	var panicVal interface{}
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer func() {
			if v := recover(); v != nil {
				panicVal = v
			}
			wg.Done()
		}()

		/* do some work, asynchronously */
		aFuncThatMayPanic()
	}(wg)

	/* do some other work */

	wg.Wait() // wait for the async work to finish
	if panicVal != nil {
		// handle the panic..
	}

	/* do some other work */
}
