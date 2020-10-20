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

	"github.com/asmsh/promise"
)

// CtxPromise is a promise with a Context value embedded inside it, and
// passed to the callback of its methods.
//
// Its method overrides the inherited methods from the embedded promise.
//
// This is just a draft implementation, a proper implementation should
// care about not passing nil callbacks, and maybe other things.
type CtxPromise struct {
	promise.Promise // always a GoPromise, in this implementation
	ctx             context.Context
}

func NewCtxPromise(resChan chan promise.Res, ctx context.Context) *CtxPromise {
	return &CtxPromise{Promise: promise.New(resChan), ctx: ctx}
}

func newCtxPromiseInter(base promise.Promise, ctx context.Context) *CtxPromise {
	return &CtxPromise{Promise: base, ctx: ctx}
}

func (cp *CtxPromise) Then(cb func(ctx context.Context, res promise.Res) promise.Res) *CtxPromise {
	p := cp.Promise.Then(func(res promise.Res, ok bool) promise.Res {
		return cb(cp.ctx, res)
	})
	newHP := newCtxPromiseInter(p, cp.ctx)
	return newHP
}

func (cp *CtxPromise) Catch(cb func(ctx context.Context, err error, res promise.Res) promise.Res) *CtxPromise {
	p := cp.Promise.Catch(func(err error, res promise.Res, ok bool) promise.Res {
		return cb(cp.ctx, err, res)
	})
	newHP := newCtxPromiseInter(p, cp.ctx)
	return newHP
}

func (cp *CtxPromise) Recover(cb func(ctx context.Context, v interface{}) promise.Res) *CtxPromise {
	p := cp.Promise.Recover(func(v interface{}, ok bool) promise.Res {
		return cb(cp.ctx, v)
	})
	newHP := newCtxPromiseInter(p, cp.ctx)
	return newHP
}

func (cp *CtxPromise) Finally(cb func(ctx context.Context, ok bool) promise.Res) *CtxPromise {
	p := cp.Promise.Finally(func(ok bool) promise.Res {
		return cb(cp.ctx, ok)
	})
	newHP := newCtxPromiseInter(p, cp.ctx)
	return newHP
}

func main() {
	resChan := make(chan promise.Res)
	ctx := NewCtxPromise(resChan, context.Background())
	ctx.Then(func(ctx context.Context, res promise.Res) promise.Res {
		return nil
	})
}
