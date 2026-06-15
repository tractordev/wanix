package main

import (
	"os"

	"tractor.dev/wanix/rc"
)

func main() {
	os.Exit(rc.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
