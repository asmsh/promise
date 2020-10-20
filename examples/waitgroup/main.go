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

// The following function shows how a Promise maybe used instead of
// a WaitGroup with only one task waiting for.
func main() {
	p := promise.Go(func() {
		/* do some work, asynchronously */
	})

	/* do some other work */

	p.Wait() // wait for the async work to finish

	/* do some other work */
}

// The following functions are equivalent to the above one(without any panic
// handling logic), but using the standard library and the language's primitives.

func stdAlt1() {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		/* do some work, asynchronously */
		wg.Done()
	}(wg)

	/* do some other work */

	wg.Wait() // wait for the async work to finish

	/* do some other work */
}

func stdAlt2() {
	wait := make(chan struct{})
	go func(wait chan struct{}) {
		defer close(wait)
		/* do some work, asynchronously */
	}(wait)

	/* do some other work */

	<-wait // wait for the async work to finish

	/* do some other work */
}
