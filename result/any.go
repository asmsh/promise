// Copyright 2023 Ahmad Sameh(asmsh)
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

package result

import "context"

func Any(vals ...any) AnyRes {
	return vals
}

// ImmutAny takes the to-be-returned values, vals, add them to a new Res value,
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
//	 // inside any callback..
//	 // ('getMyRes' is any function that returns a Res value, and
//	 // may modify the value after it's returned from this callback.)
//	 myRes := getMyRes()
//
//		/* update 'myRes' (or do other work) */
//
//	 // when returning, don't return 'myRes' directly, like:
//	 return myRes
//	 // instead, return it through ImmutAny, as follows:
//		return promise.ImmutAny(myRes...)
func ImmutAny(vals ...any) (res AnyRes) {
	if n := len(vals); n != 0 {
		res = make(AnyRes, n)
		copy(res, vals)
	}
	return
}

// ReuseAny takes a Res value, base, and repopulate it with the provided
// values, vals, only if it can hold them, and returns it.
// Otherwise, a new Res value will be allocated, populated with vals, and
// returned.
//
// The provided base will be populated with the provided values, if its
// capacity is less than or equal to the length of the provided values.
// Otherwise, it will not be touched.
//
// If no values are provided, it will return nil, without touching the base.
//
// It should be used only inside callbacks, and when the provided Res value,
// base, is not needed any more(before this call), and the number of the
// to-be-returned values is less than the base's capacity.
//
// It only serves as a way to minimize memory allocations when the number of
// values returned from a callback is the same as the length of the Res value
// passed to the callback, by reusing that Res value(under the above conditions).
// However, it will always return a Res value that contain the passed values,
// vals, so it can be used as a general return function which can, when
// appropriate, optimize memory allocations.
//
// Example:
//
//	 // inside any callback..
//	 // ('res' is the Res value passed to the callback)
//
//		/* after done needing 'res' */
//
//	 // ('val1, val2, ..., valN' are any values)
//	 // when returning the following values, val1, val2, ..., valN,
//	 // and N is less than or equal cap(res), don't return them like:
//	 return promise.Res{val1, val2, ..., valN}
//	 // instead, return them through ReuseAny, as follows:
//		return promise.ReuseAny(res, val1, val2, ..., valN)
func ReuseAny(base AnyRes, vals ...any) (res AnyRes) {
	n := len(vals)
	if n == 0 {
		// return nil if no values are passed
		return nil
	}

	// if the capacity of base can hold the passed values, vals,
	// reuse base for storing vals, otherwise a new Res value will
	// be allocated(in append, below), and without touching base.
	if n <= cap(base) {
		base.Clear()
		res = base[:0]
	}

	return append(res, vals...)
}

// AnyRes is the type of the return value returned from all callbacks.
// It represents the return parameters of functions/methods in Go, and as they
// can be multiple return parameters, the returned values are represented here
// as a slice of values.
//
// It follows the convention of passing the error, if any, as the last parameter
// in the return parameters list, which is represented here as the last element
// in the slice.
// So, if a callback returned a non-nil error value as the last element in the
// returned AnyRes value, the returned Promise will be a 'rejected' promise.
//
// Values of this type must not be modified after returned from any callback.
// TODO: maybe choose another name, something like VectorRes
type AnyRes []any

func (res AnyRes) Val() AnyRes {
	return res
}

func (res AnyRes) Err() error {
	last, ok := res.Last()
	if !ok {
		return nil
	}

	err, _ := last.(error)
	return err
}

func (res AnyRes) Ctx() context.Context {
	first, ok := res.First()
	if !ok {
		return context.Background()
	}

	ctx, ok := first.(context.Context)
	if !ok {
		return context.Background()
	}

	return ctx
}

// First returns the first element of this AnyRes value and true, if its length
// is not 0, otherwise it returns nil and false.
func (res AnyRes) First() (first any, ok bool) {
	if len(res) == 0 {
		return nil, false
	}
	return res[0], true
}

// Last returns the last element of this AnyRes value and true, if its length
// is not 0, otherwise it returns nil and false.
func (res AnyRes) Last() (last any, ok bool) {
	n := len(res)
	if n == 0 {
		return nil, false
	}
	return res[n-1], true
}

// IsZero returns true if this AnyRes value has no elements, or if all of its
// elements are nil, otherwise it returns false.
func (res AnyRes) IsZero() bool {
	n := len(res)
	if n == 0 {
		return true
	}

	for i := 0; i < n; i++ {
		if res[i] != nil {
			return false
		}
	}
	return true
}

// Copy returns a new copy of this AnyRes value if it's not empty, otherwise
// returns this AnyRes value.
// It doesn't touch the elements inside the AnyRes value.
func (res AnyRes) Copy() (newRes AnyRes) {
	n := len(res)
	if n == 0 {
		return res
	}

	newRes = make(AnyRes, n)
	copy(newRes, res)
	return newRes
}

// Clear sets all elements of this AnyRes value to nil, and returns its length.
func (res AnyRes) Clear() (n int) {
	n = len(res)
	if n == 0 {
		return 0
	}

	for i := 0; i < n; i++ {
		res[i] = nil
	}
	return n
}
