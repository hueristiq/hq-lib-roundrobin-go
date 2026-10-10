# hq-lib-roundrobin-go

![made with go](https://img.shields.io/badge/made%20with-Go-1E90FF.svg) [![go reference](https://pkg.go.dev/badge/github.com/hueristiq/hq-lib-roundrobin-go.svg)](https://pkg.go.dev/github.com/hueristiq/hq-lib-roundrobin-go) [![license](https://img.shields.io/badge/license-MIT-gray.svg?color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/LICENSE) ![maintenance](https://img.shields.io/badge/maintained%3F-yes-1E90FF.svg) [![open issues](https://img.shields.io/github/issues-raw/hueristiq/hq-lib-roundrobin-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/issues?q=is:issue+is:open) [![closed issues](https://img.shields.io/github/issues-closed-raw/hueristiq/hq-lib-roundrobin-go.svg?style=flat&color=1E90FF)](https://github.com/hueristiq/hq-lib-roundrobin-go/issues?q=is:issue+is:closed) [![contribution](https://img.shields.io/badge/contributions-welcome-1E90FF.svg)](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/CONTRIBUTING.md)

`hq-lib-roundrobin-go` is a [Go](https://golang.org/) package providing a generic, concurrency-safe [round-robin](https://en.wikipedia.org/wiki/Round-robin_scheduling) selector: it cycles through a set of items, handing out one per call, optionally serving some items more often than others through weights.

## Resources

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
	- [Basic Rotation](#basic-rotation)
	- [Rotate Amount](#rotate-amount)
	- [Weighted Selection](#weighted-selection)
	- [Statistics](#statistics)
- [Configuration](#configuration)
- [Contributing](#contributing)
- [Licensing](#licensing)

## Features

- **Generic Item Type:** `RoundRobin[T]` cycles through items of any comparable type — strings, URLs, structs, or your own values.
- **Weighted Selection:** Per-item weights switch selection to smooth weighted round-robin, serving each item in proportion to its weight with picks spread evenly instead of bunched into runs.
- **Configurable Rotation:** `WithRotateAmount` serves each item a fixed number of times in a row before advancing.
- **Concurrency-Safe:** Every method is safe for concurrent use; mutable state is guarded by a single mutex and statistics counters are atomic.
- **Deduplication & Removal:** Duplicate values are collapsed on add, and items can be removed by value at any time.
- **Per-Item Statistics:** Track and reset serve counts per item, even after an item leaves the cycle.

## Installation

To install `hq-lib-roundrobin-go`, run the following command in your Go project:

```bash
go get -v -u github.com/hueristiq/hq-lib-roundrobin-go
```

## Usage

The examples below import the package under the `hqgoroundrobin` alias.

```go
import hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
```

### Basic Rotation

Use `New` with at least one item; every call to `Next` serves the next item in the cycle, wrapping around in insertion order:

```go
package main

import (
	"fmt"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func main() {
	rr, err := hqgoroundrobin.New([]string{"server-1", "server-2", "server-3"})
	if err != nil {
		fmt.Printf("Failed to create round-robin: %v\n", err)

		return
	}

	for range 6 {
		fmt.Println(rr.Next())
	}
	// Output:
	// server-1
	// server-2
	// server-3
	// server-1
	// server-2
	// server-3
}
```

### Rotate Amount

Use `WithRotateAmount` to serve each item several times in a row before advancing — useful when each pick should be reused for a batch:

```go
package main

import (
	"fmt"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func main() {
	rr, err := hqgoroundrobin.New([]string{"a", "b"}, hqgoroundrobin.WithRotateAmount(2))
	if err != nil {
		fmt.Printf("Failed to create round-robin: %v\n", err)

		return
	}

	for range 4 {
		fmt.Println(rr.Next())
	}
	// Output:
	// a
	// a
	// b
	// b
}
```

### Weighted Selection

Use `AddWithWeight` to make some items be served more often. Once any item carries a non-default weight, selection switches to smooth weighted round-robin: over every window of `sum(weights)` selections each item is picked exactly `weight` times, spread across the window. Adding a weight of `3` to one of three items:

```go
package main

import (
	"fmt"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func main() {
	rr, err := hqgoroundrobin.New([]string{"fast", "slow"})
	if err != nil {
		fmt.Printf("Failed to create round-robin: %v\n", err)

		return
	}

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
}
```

Items can also be added with `Add`, removed by value with `Remove`, and listed with `Items`, which returns a snapshot copy:

```go
rr.Add("server-4", "server-5")
rr.Remove("server-3")

fmt.Println(rr.Items()) // snapshot; mutating it never affects the cycle
fmt.Println(rr.Len())
```

### Statistics

`StatisticsOf` returns a live, atomic view of an item's serve count:

```go
package main

import (
	"fmt"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func main() {
	rr, err := hqgoroundrobin.New([]string{"a", "b", "c"})
	if err != nil {
		fmt.Printf("Failed to create round-robin: %v\n", err)

		return
	}

	for range 7 {
		rr.Next()
	}

	if stats, found := rr.StatisticsOf("a"); found {
		fmt.Println(stats.ServesCount()) // 3

		stats.ResetServesCount()
	}
}
```

## Configuration

Behavior is set through functional options passed to `New`. Any option left unset falls back to its default.

| Option | Description | Default |
| --- | --- | --- |
| `WithRotateAmount(n)` | Number of times in a row one item is served before the selection advances to the next item. A value `<= 0` falls back to the default. | `DefaultRotateAmount` (`1`) |

Weights are not an option but a per-item property: pass them through `AddWithWeight` (a non-positive weight falls back to `1`). While every item carries the default weight, the cycle is strict rotation in insertion order; as soon as an item carries a different weight, selection becomes smooth weighted round-robin.

## Contributing

Contributions are welcome and encouraged! Feel free to submit [Pull Requests](https://github.com/hueristiq/hq-lib-roundrobin-go/pulls) or report [Issues](https://github.com/hueristiq/hq-lib-roundrobin-go/issues). For more details, check out the [contribution guidelines](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/CONTRIBUTING.md).

A big thank you to all the [contributors](https://github.com/hueristiq/hq-lib-roundrobin-go/graphs/contributors) for your ongoing support!

![contributors](https://contrib.rocks/image?repo=hueristiq/hq-lib-roundrobin-go&max=500)

## Licensing

This package is licensed under the [MIT license](https://opensource.org/license/mit). You are free to use, modify, and distribute it, as long as you follow the terms of the license. You can find the full license text in the repository - [Full MIT license text](https://github.com/hueristiq/hq-lib-roundrobin-go/blob/main/LICENSE).
