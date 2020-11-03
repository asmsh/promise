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

func AsyncGet(url string) *HttpPromise {
	resChan := make(chan promise.Res, 1)
	p := NewHttpPromise(resChan)
	go asyncGetCall(url, resChan)
	return p
}

func asyncGetCall(url string, resChan chan promise.Res) {
	resp, err := http.Get(url)
	resChan <- promise.Res{resp, err}
}

// HttpPromise implements the promise.Promise interface.
//
// Its methods are defined next to the inherited methods from the embedded
// promise.
//
// This is just a draft implementation, a proper implementation should
// care about not passing nil callbacks, and maybe other things.
type HttpPromise struct {
	promise.Promise // always a GoPromise, in this implementation
}

func NewHttpPromise(resChan chan promise.Res) *HttpPromise {
	return &HttpPromise{Promise: promise.New(resChan)}
}

func newHttpPromiseInter(base promise.Promise) *HttpPromise {
	return &HttpPromise{Promise: base}
}

func (hp *HttpPromise) GetResT() (*http.Response, error) {
	res, _ := hp.GetRes()
	resp := res[0].(*http.Response)
	err := res[1].(error)
	return resp, err
}

func (hp *HttpPromise) ThenT(cb func(resp *http.Response) (resResp *http.Response, resErr error)) *HttpPromise {
	p := hp.Then(func(res promise.Res, ok bool) promise.Res {
		resp := res[0].(*http.Response)
		resResp, resErr := cb(resp)
		return promise.ReuseRes(res, resResp, resErr) // will not create new Res value
	})
	newHP := newHttpPromiseInter(p)
	return newHP
}

func (hp *HttpPromise) CatchT(cb func(err error, resp *http.Response) (resResp *http.Response, resErr error)) *HttpPromise {
	p := hp.Catch(func(err error, res promise.Res, ok bool) promise.Res {
		resp := res[0].(*http.Response)
		resResp, resErr := cb(err, resp)
		return promise.ReuseRes(res, resResp, resErr) // will not create new Res value
	})
	newHP := newHttpPromiseInter(p)
	return newHP
}
