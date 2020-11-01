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

// Res is the type of the return value returned from all callbacks.
// It represents the return parameters of functions/methods in Go, and as they
// can be multiple return parameters, the returned values are represented here
// as a slice of values.
//
// It follows the convention of passing the error, if any, as the last parameter
// in the return parameters list, which is represented here as the last element
// in the slice.
// So, if a callback returned a non-nil error value as the last element in the
// returned Res value, the returned Promise will be a 'rejected' promise.
//
// Values of this type must not be modified after returned from any callback.
type Res []interface{}

// First returns the first element of this Res value and true, if its length
// is not 0, otherwise it returns nil and false.
func (res Res) First() (first interface{}, ok bool) {
	if len(res) == 0 {
		return nil, false
	}
	return res[0], true
}

// Last returns the last element of this Res value and true, if its length
// is not 0, otherwise it returns nil and false.
func (res Res) Last() (last interface{}, ok bool) {
	n := len(res)
	if n == 0 {
		return nil, false
	}
	return res[n-1], true
}

// GetErr returns the last element of this Res value, if its type is error.
// It returns nil if this Res value is empty or the type of the last element
// is not error.
func (res Res) GetErr() error {
	last, ok := res.Last()
	if !ok {
		return nil
	}

	if err, ok := last.(error); ok {
		return err
	}
	return nil
}

// IsErrRes returns true if this Res value represents an error result, which
// is a result that has a non-nil error as its last element.
func (res Res) IsErrRes() bool {
	return res.GetErr() != nil
}

// Copy returns a new copy of this Res value if it's not empty, otherwise
// returns this Res value.
func (res Res) Copy() (newRes Res) {
	n := len(res)
	if n == 0 {
		return res
	}

	newRes = make(Res, n)
	copy(newRes, res)
	return newRes
}

// Clear sets all elements of this Res value to nil, and returns its length.
func (res Res) Clear() (n int) {
	n = len(res)
	if n == 0 {
		return 0
	}

	for i := 0; i < n; i++ {
		res[i] = nil
	}
	return n
}
