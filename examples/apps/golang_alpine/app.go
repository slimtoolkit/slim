package main

import (
	"fmt"
	"runtime"
)

func main() {
  fmt.Printf("[%v] Sample golang app (alpine)\n",runtime.Version())
}
