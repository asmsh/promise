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
	"context"
	"fmt"
	"net/http"

	"github.com/asmsh/promise"
)

func main() {
	AsyncGet("https://golang.org/").
		Follow(func(ctx context.Context, res promise.Result[*http.Response]) promise.Result[*http.Response] {
			if res.State() != promise.Success {
				return res
			}
			resp := res.Val()

			// do something with resp..
			fmt.Printf("resp: %v\n", resp)

			return nil
		}).
		Follow(func(ctx context.Context, res promise.Result[*http.Response]) promise.Result[*http.Response] {
			if res.State() != promise.Error {
				return res
			}
			err := res.Err()

			// handle the error..
			fmt.Printf("err: %v\n", err)

			return nil
		}).
		Wait() // wait the whole pipeline to finish
}
