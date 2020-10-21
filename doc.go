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

// Package promise provides fast, lightweight, and lock-free promise implementations.
//
// Besides speed and low memory overhead, it focuses on Go's idioms, like multi
// return parameters, error being a value, the convention of returning errors as
// the last return parameter, and the existence of panics and their difference
// from errors.
//
// A Promise provides an easy way for returning results from a goroutine, and/or
// waiting for it to finish, plus other features.
//
// A Promise has four states, and it can be in only one of them, at any time:
// Pending: the computation that corresponds to this Promise has not finished.
// Fulfilled: the computation that corresponds to this Promise has finished and
// returned a Res value with no error or with a nil error at the end.
// Rejected: the computation that corresponds to this Promise has finished and
// returned a Res value with a non-nil error at the end.
// Panicked: the computation that corresponds to this Promise has caused a panic.
//
// A Promise has three fates, and it can be in only one of them, at any time:
// Unresolved: the computation that corresponds to this Promise is still working,
// and the final state of the Promise is still unknown.
// Resolved: the state of the Promise is now known, and final.
// Handled: the result of the Promise has been passed to some call of its methods.
//
//
// General Notes:-
//
// * Once the Promise's fate is Resolved, its result value will not change.
//
// * A Promise whose fate is Unresolved, its state must be Pending.
//
// * A Promise whose fate is Resolved, and its state is Pending, its fate will
// never be Handled.
//
// * For a Promise's fate to be Handled, its fate must first be Resolved.
//
//
// Implementations:-
//
// * The module provides one Promise implementation, GoPromise(the default).
//
// * The GoPromise's 'ok' callback parameter will always be true(except in
// a Finally callback on a panicked promise).
//
// * The GoPromise's 'ok' return parameter(for GetRes and GetResUntil) will
// always be true(except on a panicked promise)
//
//
// Callback Notes:-
//
// * The Res value returned from the callback must not be modified after return.
//
// * If the callback called runtime.Goexit, the returned promise will be
// fulfilled to 'nil'.
//
// * The underlying implementation of the returned Promise is the same as
// the receiver promise's.
//
// * If the callback returned a non-nil error value as the last element in
// the returned Res value, the returned Promise will be a 'rejected' promise,
// and all subsequent promises in any promise chain derived from it, until
// the error is caught on each of these chains(by a Catch call).
//
// * If the promise is running in the safe mode(the default), the returned
// Promise is a rejected promise, and the error is not caught(by a Catch call)
// before the end of that promise's chain, a panic will happen with an error
// value of type *UnCaughtErr, which has that uncaught error 'wrapped' inside it.
//
// * If the callback caused a panic, the resulting Promise will be a 'panicked'
// promise, and all subsequent promises in any promise chain derived from it,
// until recovering from the panic on each of these chains(by a Recover call).
//
// * If the returned Promise is a panicked promise, and it's not recovered(by a
// Recover call) before the end of the promise's chain, or before calling Finally,
// it will re-panic(with the same value passed to the original 'panic' call).
//
//
// Modes:-
//
// * The module provides two modes for promise creation, 'Safe', and 'NonSafe'.
//
// * In the 'NonSafe' version, a rejected promise(resolved to the 'rejected'
// state) will not panic if it reached the end of its promise chain without
// being caught(by a 'Catch' call). But in the 'Safe' version a panic will
// happen in that case.
//
// * The 'NonSafe' version is accessible from the NonSafeAPI variable.
//
// * The 'Safe' version is the default, and is accessible from any function
// in the module.
package promise
