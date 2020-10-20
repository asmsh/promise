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
)

func main() {
	p := AsyncGet("https://golang.org/")
	p.ThenT(func(resp *http.Response) (resResp *http.Response, resErr error) {
		// do something with resp..
		fmt.Printf("resp: %v\n", resp)
		return
	}).CatchT(func(err error, resp *http.Response) (resResp *http.Response, resErr error) {
		// handle the error..
		fmt.Printf("err: %v\n", err)
		return
	}).Wait() // wait the whole pipeline to finish
}
