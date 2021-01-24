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

package promise

import "time"

// NonSafeAPI exposes the 'NonSafe' version of the API of this module, as
// the 'Safe' version is the default and already accessible directly from
// this module.
//
// All promises created through this will be 'NonSafe', and all promises
// that are derived from them too.
//
// In the 'NonSafe' version, a 'rejected' or 'panicked' promise(resolved
// to the 'rejected' state or the 'panicked' state, respectively) will not
// panic if it reached the end of its promise chain without being handled(
// by a 'Catch' call or 'GetRes' call, in case of a 'rejected' promise, or
// a 'Recover' call, in case of 'panicked' promise).
// But in the 'Safe' version a panic will happen in either of these cases.
//
// It should be used with care, as errors and panics might be returned or
// occurred and go unnoticed.
//
// It is an extension variable that's sole purpose is to organize the API
// of the module, and encourage the use of the 'Safe' version of API.
var NonSafeAPI nonSafeCalls

// dummy type with private field to make it unique than the empty struct,
// and disallow changing the NonSafeAPI value
type nonSafeCalls struct{ i int }

func (nonSafeCalls) Go(fun func()) *GoPromise {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}
	prom := newGoPromInter(true)
	go goCall(prom, fun)
	return prom
}

func (nonSafeCalls) GoRes(fun func() Res) *GoPromise {
	if fun == nil {
		panic(nilCallbackPanicMsg)
	}
	prom := newGoPromInter(true)
	go goResCall(prom, fun)
	return prom
}

func (nonSafeCalls) New(resChan chan Res) *GoPromise {
	prom := newGoPromExter(resChan, true)
	return prom
}

func (nonSafeCalls) Resolver(resolverCb func(fulfill func(vals ...interface{}), reject func(err error, vals ...interface{}))) *GoPromise {
	if resolverCb == nil {
		panic(nilCallbackPanicMsg)
	}
	prom := newGoPromInter(true)
	go resolverCall(prom, resolverCb)
	return prom
}

func (nonSafeCalls) Wrap(res Res) *GoPromise {
	prom := newGoPromSync(true)
	wrapCall(prom, res)
	return prom
}

func (nonSafeCalls) Delay(res Res, d time.Duration, onSucceed, onFail bool) *GoPromise {
	prom := newGoPromInter(true)
	go delayCall(prom, res, d, onSucceed, onFail)
	return prom
}

func (nonSafeCalls) Fulfill(vals ...interface{}) *GoPromise {
	newProm := newGoPromSync(true)
	newProm.fulfillSync(vals)
	return newProm
}

func (nonSafeCalls) Reject(err error, vals ...interface{}) *GoPromise {
	prom := newGoPromSync(true)
	if len(vals) == 0 {
		prom.rejectSync(Res{err})
		return prom
	}
	prom.rejectSync(append(vals, err))
	return prom
}

func (nonSafeCalls) Panic(v interface{}) *GoPromise {
	prom := newGoPromSync(true)
	prom.panicSync(Res{v})
	return prom
}
