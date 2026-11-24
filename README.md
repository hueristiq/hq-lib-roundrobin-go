# hq-lib-roundrobin-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go reference](https://pkg.go.dev/badge/github.com/hueristiq/hq-lib-roundrobin-go.svg)](https://pkg.go.dev/github.com/hueristiq/hq-lib-roundrobin-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-roundrobin-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-roundrobin-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/issues?q=is:issue+is:closed)

`hq-lib-roundrobin-go` is a [Go](https://golang.org/) package for a generic, concurrency-safe [round-robin](https://en.wikipedia.org/wiki/Round-robin_scheduling) selector.

## Resources

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Basic rotation](#basic-rotation)
	- [Rotate amount](#rotate-amount)
	- [Weighted selection](#weighted-selection)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Selection:** `RoundRobin[T]` cycles through any comparable type.
- **Weights:** `AddWithWeight` enables smooth weighted selection.
- **Rotation:** `SetRotateAmount` repeats each item before advancing.
- **Membership:** duplicate adds collapse; `Remove` deletes by value.
- **Concurrency:** all methods are safe for concurrent use.

## Installation

To install `hq-lib-roundrobin-go`, run the following command in your Go project:

```bash
go get github.com/hueristiq/hq-lib-roundrobin-go@latest
```

Requires Go 1.26.0 or later.

## Usage

The examples import the package under the `hqgoroundrobin` alias.

### Basic rotation

`New` takes the initial items directly. Every call to `Next` serves the next item in the cycle, wrapping around in insertion order.

```go
package main

import (
	"fmt"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func main() {
	rr := hqgoroundrobin.New("server-1", "server-2", "server-3")

	fmt.Println(rr.Next())
	fmt.Println(rr.Next())
	fmt.Println(rr.Next())
	fmt.Println(rr.Next())

	rr.Add("server-4")
	rr.Remove("server-2")

	fmt.Println(rr.Items())
	fmt.Println(rr.Len())
}
```

`Add` appends items, `Remove` deletes a value, and `Items` returns a snapshot copy. Mutating the snapshot never affects the cycle.

### Rotate amount

Use `SetRotateAmount` to serve each item several times in a row before advancing. This helps when each pick should be reused for a batch.

```go
rr := hqgoroundrobin.New("a", "b")

rr.SetRotateAmount(2)

fmt.Println(rr.Next()) // a
fmt.Println(rr.Next()) // a
fmt.Println(rr.Next()) // b
fmt.Println(rr.Next()) // b
```

### Weighted selection

Use `AddWithWeight` to make some items serve more often. Once any item carries a non-default weight, selection switches to smooth weighted round-robin: in the steady state each item is picked in proportion to its weight, with picks spread across the cycle. When every item again carries the default weight, selection returns to strict rotation in insertion order.

```go
rr := hqgoroundrobin.New("fast", "slow")

rr.AddWithWeight("fast", 3) // Re-adding an existing value updates its weight.

for range 8 {
	fmt.Println(rr.Next())
}
```

An empty cycle is legal. `Next` returns the zero value of `T` and reports whether a serve was possible.

```go
rr := hqgoroundrobin.New[string]()

if value, ok := rr.Next(); ok {
	fmt.Println(value)
} else {
	fmt.Println("cycle is empty")
}
```

### Concurrent use

All methods are safe for concurrent use.

```go
rr := hqgoroundrobin.New("a", "b")

var wg sync.WaitGroup

for range 10 {
	wg.Add(1)

	go func() {
		defer wg.Done()

		rr.Next()
	}()
}

wg.Wait()
```

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-roundrobin-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-roundrobin-go/issues).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-roundrobin-go/graphs/contributors) for your ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-roundrobin-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/LICENSE).
