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

// MustGetRes calls GetRes on the provided promise, and returns its result,
// only if the returned ok value = true, otherwise, it panics.
//
// By name convention, the function will return the result successfully(ok =
// true), or a panic will happen.
// This functions should be used on promises which their GetRes methods are
// known to always return ok = true(GoPromise with no 'Recover' follow).
func MustGetRes(p Promise) (res Res) {
	res, ok := p.GetRes()
	if !ok {
		panic("promise: GetRes returned ok = false")
	}
	return res
}

// ImmutRes takes the to-be-returned values, add them to a new Res value,
// and returns the newly allocated Res value.
//
// It effectively copies the provided slice of values, vals, into a new Res
// value, so if it's passed with a slice, the returned Res value will be
// immutable, meaning that, modifying the passed slice will not modify the
// returned Res value.
//
// It should be used only inside callbacks, and when the to-be-returned Res
// value is not created at return site and maybe modified after return, or
// when that Res value is retrieved from another function that may modify
// it after return.
//
// Example:
//
//  // inside any callback..
//  // ('getMyRes' is any function that returns a Res value, and
//  // may modify the value after it's returned from this callback.)
//  myRes := getMyRes()
//
//	/* update myRes (or do other work) */
//
//  // when returning, don't return 'myRes' directly, like:
//  // "return myRes"
//  // instead, return it through ImmutRes, as follows..
//	return promise.ImmutRes(myRes...)
//
func ImmutRes(vals ...interface{}) (res Res) {
	if n := len(vals); n != 0 {
		res = make(Res, n)
		copy(res, vals)
	}
	return
}

// WaitAll waits all the provided promises to resolve then return true, or
// returns false if no promises are provided.
func WaitAll(proms ...Promise) (waited bool) {
	n := len(proms)
	if n == 0 {
		return false
	}

	for _, p := range proms {
		p.Wait()
	}
	return true
}
