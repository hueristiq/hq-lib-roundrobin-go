package main

import (
	"fmt"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func main() {
	selector := hqgoroundrobin.New("alpha", "beta")
	selector.Remove("alpha")

	value, ok := selector.NextOK()

	fmt.Println(value, ok)
}
