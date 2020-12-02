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
	"fmt"
	"time"

	"github.com/asmsh/promise"
)

// The following examples shows how to wait for a Promise or get its result
// with a timeout.
func main() {
	example1()
	example2()
}

// example1 waits on a promise with a specified timeout
func example1() {
	p := promise.Go(func() {
		/* do some work, asynchronously */
		time.Sleep(time.Millisecond * 1) // simulates some work
	})

	// create a timer channel with the needed timeout duration
	timeoutChan := time.After(time.Millisecond * 10)
	// create a wait channel for this promise
	waitChan := p.WaitChan()

	// wait the promise to be resolved with the needed timeout duration
	select {
	case ok := <-waitChan:
		// the promise is resolved before the timeout passes
		fmt.Println("The promise is resolved with ok =", ok)
	case t := <-timeoutChan:
		// the needed timeout duration has passed before the promise is resolved
		fmt.Println("The promise is not resolved yet, at time =", t)
	}

	/* do some other work */
}

// example2 waits on a promise with a specified timeout, and prints its result
// if this promise is resolved on time.
func example2() {
	p := promise.GoRes(func() promise.Res {
		/* do some work, asynchronously */
		time.Sleep(time.Millisecond * 1) // simulates some work
		return promise.Res{"go", "golang"}
	})

	// create a timer channel with the needed timeout duration
	timeoutChan := time.After(time.Millisecond * 10)
	// create a wait channel for this promise
	waitChan := p.WaitChan()

	// wait the promise to be resolved with the needed timeout duration
	select {
	case ok := <-waitChan:
		// the promise is resolved before the timeout passes
		fmt.Println("The promise is resolved with ok =", ok)

		// handle the result of the promise, as it's now resolved.
		// ok will always = true here in this example.
		res, ok := p.GetRes()
		fmt.Println("The promise's result =", res)
	case t := <-timeoutChan:
		// the needed timeout duration has passed before the promise is resolved
		fmt.Println("The promise is not resolved yet, at time =", t)
	}

	/* do some other work */
}
