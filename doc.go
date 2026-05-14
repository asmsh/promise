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
// It hides away the complexity of managing goroutines and channels for complex tasks,
// providing a simple and idiomatic API for asynchronous programming.
//
// A [Promise] represents the eventual result of an asynchronous operation.
//
// A [Group] allows managing collections of related promises. It provides high-level
// operations like [Group.Wait], [Group.AllRes], and [Group.AnyRes],
// facilitating complex coordination patterns.
//
// The package provides several ways to start promises, most notably using [Go], [GoErr],
// [GoValErr] and [Chan] functions.
// These functions manage goroutine execution and lifecycle, ensuring that results, errors,
// and panics are correctly captured and passed to the following callbacks and methods.
//
// Chaining methods like [Then], [Catch], and [Recover] allow building sequential
// asynchronous pipelines where each step can depend on the outcome of the previous one.
package promise
