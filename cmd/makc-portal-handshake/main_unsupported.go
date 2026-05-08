//go:build !linux

package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Fprintf(os.Stderr, "makc-portal-handshake is only available on Linux, not %s\n", runtime.GOOS)
	os.Exit(1)
}
