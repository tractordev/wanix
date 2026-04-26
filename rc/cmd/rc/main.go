package main

import (
	"os"

	"tractor.dev/wanix/rc/shell"
)

func main() {
	os.Exit(shell.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
