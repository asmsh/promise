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

	"github.com/asmsh/promise"
)

func main() {
	// the two examples are equal
	example1()
	example2()
}

// this does exactly as example2
func example1() {
	p := promise.Resolver(func(fulfill func(...interface{}), reject func(error, ...interface{})) {
		resp, err := http.Get("https://golang.org/")
		if err != nil {
			// pass resp also, to not break code that depends on the number of
			// return values.
			// other use cases may need to do the same, if the value returned
			// with the error(resp in this case) provides meaningful value even
			// in case of errors, like with io.EOF errors.
			reject(err, resp)
		} else {
			// reserve a place for the error(with nil), to not break code that
			// depends on the number of return values.
			fulfill(resp, nil)
		}
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
	p.Wait() // wait the whole pipeline to finish
}

// this does exactly as example1
func example2() {
	p := promise.Resolver(func(fulfill func(...interface{}), reject func(error, ...interface{})) {
		resp, err := http.Get("https://golang.org/")
		// this single reject call handles both, fulfilment and rejection.
		// it's passed with the error and the response, as it handles nil
		// error by itself and produces a fulfilled promise in that case,
		// and if the error is non nil, it produces a rejected promise.
		reject(err, resp)
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
	p.Wait() // wait the whole pipeline to finish
}
