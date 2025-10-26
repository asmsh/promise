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
	"net/http"

	"github.com/asmsh/promise"
)

// AsyncGet executes a GET request for the provided url in a new goroutine,
// and returns a [*promise.Promise] value tracking its progress.
//
// This is just a draft implementation showing the use of the [promise.GoCtxRes] function.
func AsyncGet(url string) *promise.Promise[*http.Response] {
	return promise.GoValErr(func() (*http.Response, error) {
		return http.Get(url)
	})
}
