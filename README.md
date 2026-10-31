# hq-lib-roundrobin-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go reference](https://pkg.go.dev/badge/github.com/hueristiq/hq-lib-roundrobin-go.svg)](https://pkg.go.dev/github.com/hueristiq/hq-lib-roundrobin-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-roundrobin-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-roundrobin-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/issues?q=is:issue+is:closed) [![contribution](https://img.shields.io/badge/contributions-welcome-1E90FF.svg)](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/CONTRIBUTING.md)

`hq-lib-roundrobin-go` is a [Go](https://golang.org/) package for a generic, concurrency-safe [round-robin](https://en.wikipedia.org/wiki/Round-robin_scheduling) selector.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Basic Rotation](#basic-rotation)
	- [Rotate Amount](#rotate-amount)
	- [Weighted Selection](#weighted-selection)
	- [Statistics](#statistics)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Generic Item Type:** `RoundRobin[T]` cycles through items of any comparable type — strings, URLs, structs, or your own values.
- **Weighted Selection:** Per-item weights switch selection to smooth weighted round-robin, serving each item in proportion to its weight with picks spread evenly instead of bunched into runs.
- **Configurable Rotation:** `SetRotateAmount` serves each item a fixed number of times in a row before advancing.
- **Concurrency-Safe:** Every method is safe for concurrent use; mutable state is guarded by a single mutex and statistics counters are atomic.
- **Deduplication & Removal:** Duplicate values are collapsed on add, and items can be removed by value at any time.
- **Per-Item Statistics:** Track and reset serve counts per item, even after an item leaves the cycle.

## Installation

```bash
go get github.com/hueristiq/hq-lib-roundrobin-go@latest
```

Requires Go 1.23 or later.

## Usage

The examples import the package under the `hqgoroundrobin` alias:

```go
import hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
```

### Basic Rotation

`New` takes the initial items directly; every call to `Next` serves the next item in the cycle, wrapping around in insertion order:

```go
rr := hqgoroundrobin.New("server-1", "server-2", "server-3")

fmt.Println(rr.Next()) // server-1
fmt.Println(rr.Next()) // server-2
fmt.Println(rr.Next()) // server-3
fmt.Println(rr.Next()) // server-1
```

Items can also be added with `Add`, removed by value with `Remove`, and listed with `Items`, which returns a snapshot copy:

```go
rr.Add("server-4", "server-5")
rr.Remove("server-3")

fmt.Println(rr.Items()) // snapshot; mutating it never affects the cycle
fmt.Println(rr.Len())
```

### Rotate Amount

Use `SetRotateAmount` to serve each item several times in a row before advancing — useful when each pick should be reused for a batch:

```go
rr := hqgoroundrobin.New("a", "b")

rr.SetRotateAmount(2)

fmt.Println(rr.Next()) // a
fmt.Println(rr.Next()) // a
fmt.Println(rr.Next()) // b
fmt.Println(rr.Next()) // b
```

### Weighted Selection

Use `AddWithWeight` to make some items be served more often. Once any item carries a non-default weight, selection switches to smooth weighted round-robin: in the steady state each item is picked in proportion to its `weight`, with picks spread across the cycle. When every item again carries the default weight — weights reset to `1`, or the weighted items removed — selection returns to strict rotation in insertion order. Adding a weight of `3` to one of two items:

```go
rr := hqgoroundrobin.New("fast", "slow")

rr.AddWithWeight("fast", 3) // re-adding an existing value updates its weight

for range 8 {
	fmt.Println(rr.Next())
}
// Output:
// fast
// fast
// slow
// fast
// fast
// fast
// slow
// fast
```

`NewWeighted` builds a weighted cycle in one call, drawing the initial weights from a map:

```go
rr := hqgoroundrobin.NewWeighted(map[string]int{
	"fast": 3,
	"slow": 1,
})
```

Map iteration order is undefined; call `Add` first when a deterministic initial cycle order matters.

### Statistics

`StatisticsOf` returns a live, atomic view of an item's serve count:

```go
rr := hqgoroundrobin.New("a", "b", "c")

for range 7 {
	rr.Next()
}

if stats, found := rr.StatisticsOf("a"); found {
	fmt.Println(stats.ServesCount()) // 3

	stats.ResetServesCount()
}
```

An empty cycle is legal — `Next` returns the zero value of `T` and `NextOK` reports whether a serve was possible:

```go
rr := hqgoroundrobin.New[string]()

if value, ok := rr.NextOK(); ok {
	fmt.Println(value)

	return
}

fmt.Println("cycle is empty")
```

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-roundrobin-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-roundrobin-go/issues). For more details, check out the [contribution guidelines](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/CONTRIBUTING.md).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-roundrobin-go/graphs/contributors) for their ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-roundrobin-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/LICENSE).
