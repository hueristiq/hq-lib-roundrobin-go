package main

import (
	"fmt"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func main() {
	selector := hqgoroundrobin.New("alpha", "beta", "gamma")

	for range 5 {
		fmt.Println(selector.Next())
	}
}
