# Promise

[![PkgGoDev](https://pkg.go.dev/badge/github.com/asmsh/promise)](https://pkg.go.dev/github.com/asmsh/promise)
[![Go Report Card](https://goreportcard.com/badge/github.com/asmsh/promise)](https://goreportcard.com/report/github.com/asmsh/promise)
[![Tests](https://github.com/asmsh/promise/workflows/Tests/badge.svg)](https://github.com/asmsh/promise/actions)

_Fast, lightweight, and type-safe promises for Go... the Go way_

## Features

- **Type-Safe**: Leverages Go generics for compile-time type safety.
- **Performance-First**: Lock-free implementation optimized for high-throughput asynchronous workflows.
- **Go-Centric API**: Focuses on Go idioms (multi-return parameters, error values, panic handling).
- **Automatic Panic Recovery**: Built-in mechanisms to catch and handle goroutine panics safely.
- **Comprehensive Toolset**: Support for chaining, grouping, delayed execution, and context integration.

## Installation

```bash
go get github.com/asmsh/promise
```

### Prerequisites

- Go 1.23+

## Usage

### Starting a Promise

#### Simple goroutine (no return value):

```go
p := promise.Go(func() {
    fmt.Println("Working...")
})
p.Wait()
```

#### Goroutine with a typed return value:

```go
p := promise.GoFunc[string, any](func() (string, error) {
    return "Hello, Gopher!", nil
})

res := p.WaitRes()
fmt.Println(res.Val(), res.Err())
```

### Chaining

Promises can be chained to build sequential asynchronous pipelines using `Follow` (returns a new `Promise`) and `Callback` (fire-and-forget):

```go
promise.GoFunc[int, any](func() (int, error) {
    return 10, nil
}).Follow(func(ctx context.Context, res promise.Result[int]) promise.Result[int] {
    if res.State() != promise.Success {
        fmt.Println("Error:", res.Err())
        return res
    }
    return promise.ValRes(res.Val() * 2)
}).Callback(func(ctx context.Context, res promise.Result[int]) {
    fmt.Println("Done, final state:", res.State())
})
```

### Grouping

A `Group` manages multiple related promises sharing a goroutine pool:

```go
g := promise.NewGroup[string]()

g.GoValErr(func() (string, error) { return "task 1", nil })
g.GoValErr(func() (string, error) { return "task 2", nil })

// Block until all finish and collect every result
allRes := g.AllWaitRes()
for _, r := range allRes.Val() {
    fmt.Println(r.Val())
}
```

### Combining Independent Promises

Several functions combine multiple unrelated promises:

```go
p1 := promise.GoFunc[string, any](func() (string, error) { return "one", nil })
p2 := promise.GoFunc[string, any](func() (string, error) { return "two", nil })
p3 := promise.GoFunc[string, any](func() (string, error) { return "three", nil })

// Wait for all and collect every result
joined := promise.Join(p1, p2, p3)
for _, r := range joined.WaitRes().Val() {
    fmt.Println(r.Val())
}
```

Other combinators: `All` (short-circuits on failure), `AllWait`, `Any` (short-circuits on first success), `AnyWait`, `Select` (first to resolve).

## Advanced Examples

### Asynchronous HTTP Request

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "github.com/asmsh/promise"
)

func main() {
    promise.GoFunc[*http.Response, any](func(ctx context.Context) (*http.Response, error) {
        req, _ := http.NewRequestWithContext(ctx, "GET", "https://go.dev", nil)
        return http.DefaultClient.Do(req)
    }).Follow(func(ctx context.Context, res promise.Result[*http.Response]) promise.Result[*http.Response] {
        if res.State() != promise.Success {
            fmt.Println("Error:", res.Err())
            return res
        }
        resp := res.Val()
        fmt.Println("Status:", resp.Status)
        resp.Body.Close()
        return res
    }).Wait()
}
```

## Comparisons

| Feature            |   `promise`   | `sync.WaitGroup` |  Raw Channels  |
|:-------------------|:-------------:|:----------------:|:--------------:|
| **Return Values**  | ✅ Yes (Typed) |       ❌ No       | ✅ Yes (Manual) |
| **Error Handling** | ✅ First-class |       ❌ No       |    ✅ Manual    |
| **Chaining**       |  ✅ Built-in   |       ❌ No       |      ❌ No      |
| **Panic Safety**   |  ✅ Automatic  |       ❌ No       |      ❌ No      |
| **Complexity**     |      Low      |       Low        |  Medium/High   |
