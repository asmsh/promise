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
	"net/http"
	"sync"

	"github.com/asmsh/promise"
)

// The following main function shows how a Promise maybe used to return
// a result from a 'go' call, in a much simpler way than how it can be
// done using the standard way(library).
func main() {
	p := promise.GoRes(func() promise.Res {
		resp, err := http.Get("https://golang.org/")
		return promise.Res{resp, err}
	}).Then(func(res promise.Res, ok bool) promise.Res {
		// do something with resp..
		resp := res[0].(*http.Response)
		fmt.Printf("resp: %v\n", resp)
		return nil
	}).Catch(func(err error, res promise.Res, ok bool) promise.Res {
		// handle the error..
		fmt.Printf("err: %v\n", err)
		return nil
	})

	/* do some other work */

	p.Wait() // wait for all the work to finish

	/* do some other work */
}

// The following functions are equivalent to the above one(without any panic
// handling logic), but using the standard library and the language's primitives.

func stdAlt1() {
	wg := &sync.WaitGroup{}
	var resp *http.Response
	var err error

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		resp, err = http.Get("https://golang.org/")
		wg.Done()
	}(wg)

	/* do some other work */

	wg.Wait() // wait for the async http request to finish
	if err != nil {
		// handle the error..
		fmt.Printf("err: %v\n", err)
		return
	}
	// do something with resp..
	fmt.Printf("resp: %v\n", resp)

	/* do some other work */
}

func stdAlt2() {
	type result struct {
		resp *http.Response
		err  error
	}
	resChan := make(chan result)

	go func(resChan chan result) {
		resp, err := http.Get("https://golang.org/")
		resChan <- result{resp: resp, err: err}
	}(resChan)

	/* do some other work */

	res := <-resChan // wait for the async http request to finish
	if res.err != nil {
		// handle the error..
		fmt.Printf("err: %v\n", res.err)
		return
	}
	// do something with res.resp..
	fmt.Printf("resp: %v\n", res.resp)

	/* do some other work */
}
