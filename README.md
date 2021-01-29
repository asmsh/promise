# Promise [![PkgGoDev](https://pkg.go.dev/badge/github.com/asmsh/promise)](https://pkg.go.dev/github.com/asmsh/promise) [![Go Report Card](https://goreportcard.com/badge/github.com/asmsh/promise)](https://goreportcard.com/report/github.com/asmsh/promise) [![GoCoverIO](https://gocover.io/_badge/github.com/asmsh/promise)](https://gocover.io/github.com/asmsh/promise) [![Tests](https://github.com/asmsh/promise/workflows/Tests/badge.svg)](https://github.com/asmsh/promise/actions)

*Fast, lightweight, and lock-free promises for Go... the Go way*

## Features

* Fast and low memory overhead implementation
* Lock-free implementation
* Extensible implementation that can be used to provide typed promises
* Automatic panic recovering
* An API that focuses on Go's idioms, like:
	* Multi return parameters
	* Error being a value
	* The convention of returning errors as the last return parameter
	* The existence of panics and their difference from errors.

## Overview

### What's a Promise ?

A Promise represents some asynchronous work. It offers ways to get the eventual result of that asynchronous work.  
Read more on [its Wiki page](https://en.wikipedia.org/wiki/Futures_and_promises).

### Why in Go ?

Waiting for a *goroutine*, or returning result from a *goroutine*, specially with multi return parameters, can get
tedious, as it usually requires using more than one variable(a channel with a *struct* type whose fields correspond to
the return parameters, or a `sync.WaitGroup` with some variables).

But using a promise, it's now possible to wait for the corresponding *goroutine* to finish and/or access the returned
result from it, along with other features, like knowing the status of that *goroutine*, building computation pipelines,
and recovering from panics.

## Installation

> go get github.com/asmsh/promise

**Note:** some breaking changes maybe present between each minor version, until the release of v1.0.0, so always check
the changelog before updating.

### Prerequisites

Go 1.13+

## Usage

### Basic Usage:

* If you want to return some values from a *goroutine*:

```go
p := promise.GoRes(func () promise.Res {
	/* do some work, asynchronously */
	return promise.Res{"go", "golang"} // return any values(as elements)
})
```

* If you don't want to return any values:

```go
p := promise.Go(func () {
	/* do some work, asynchronously */
})
```

Then use any of `p`'s methods to wait for it, access its result, or create a pipeline.

## Examples

* The simplest use case, is to use it as a `sync.WaitGroup` with only one task:

```go
package main

import "github.com/asmsh/promise"

func main() {
	p := promise.Go(func() {
		/* do some work, asynchronously */
	})

	/* do some other work */

	p.Wait() // wait for the async work to finish

	/* do some other work */
}

```

The following code is equivalent to the previous, but using only the standard library:

```go
package main

import "sync"

func main() {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		/* do some work, asynchronously */
		wg.Done()
	}(wg)

	/* do some other work */

	wg.Wait() // wait for the async work to finish

	/* do some other work */
}
```

* A typical use case, is to use it to return some result from a goroutine:

```go
package main

import (
	"net/http"

	"github.com/asmsh/promise"
)

func main() {
	p := promise.GoRes(func() promise.Res {
		resp, err := http.Get("https://golang.org/")
		return promise.Res{resp, err}
	}).Then(func(res promise.Res, ok bool) promise.Res {
		// uncomment the following and do something with resp..
		//resp := res[0].(*http.Response)
		return nil
	}).Catch(func(err error, res promise.Res, ok bool) promise.Res {
		// handle the error..
		return nil
	})

	/* do some other work */

	p.Wait() // wait for all the work to finish

	/* do some other work */
}
```

* It can be embedded or extended, to provide typed promises, like
  the [asyncHttp](https://github.com/asmsh/promise/tree/main/examples/asyncHttp), or
  the [ctxProm](https://github.com/asmsh/promise/tree/main/examples/ctxProm) examples.

* It provides JavaScript-like Resolver constructor, which can be used like
  in [this example.](https://github.com/asmsh/promise/tree/main/examples/resolver)
