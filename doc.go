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

// Package promise provides fast, lightweight, and type-safe promise implementation for Go.
// It hides away the complexity of managing goroutines and channels — including returning
// results from them — providing a simple and idiomatic API for asynchronous programming.
//
// # Core types
//
// A [Result] is a container for the result of an asynchronous operation,
// holding a value, an error, and a [State].
//
// A [Promise] represents an asynchronous operation. It can be awaited via
// [Promise.Wait], [Promise.WaitRes], or [Promise.WaitChan].
//
// A [Group] manages a collection of related promises sharing a goroutine pool
// and providing coordination operations like [Group.AllWaitRes] and [Group.AnyRes].
//
// # Starting promises
//
// Package-level constructors start new promises without a [Group]:
//
//   - [Go]: runs a func() in a goroutine.
//   - [GoFunc]: runs any callback matching the [Func] constraint.
//   - [Chan]: resolves once a value is sent on a channel.
//   - [Ctx]: resolves once a [context.Context] is canceled.
//   - [Delay]: resolves a [Result] after a duration.
//   - [Wrap]: wraps an existing [Result] synchronously.
//
// [Group] provides the same constructors (except [GoFunc]), plus [Group.GoErr],
// [Group.GoValErr], [Group.GoCtxErr], [Group.GoCtxValErr], and [Group.GoCtxRes]
// for additional callback signatures.
//
// # Chaining
//
// Promises can be chained to build sequential asynchronous pipelines:
//
//   - [Promise.Follow]: runs a callback when the promise resolves, returning a new [Promise].
//   - [Promise.Callback]: runs a fire-and-forget callback when the promise resolves.
//   - [Promise.Delay]: delays the resolution of the promise by a duration.
//   - [Follow]: type-converting follow that allows changing the result type between steps.
//
// # Combining promises
//
// Several functions combine multiple promises:
//
//   - [Wait]: blocks until all promises resolve.
//   - [Select]: resolves to the first resolved promise.
//   - [All]: resolves to [Success] when all succeed, or short-circuits on failure.
//   - [AllWait]: waits for all to resolve, then resolves to [Success] if all succeeded.
//   - [Any]: resolves to [Success] once one succeeds, or fails if all fail.
//   - [AnyWait]: waits for all to resolve, then resolves to [Success] if any succeeded.
//   - [Join]: waits for all to resolve and collects every result.
package promise
