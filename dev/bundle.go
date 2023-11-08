//go:build bundle

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Writing self-contained bootloader to ./local/wanix-bootloader.js ...")

	f, err := os.Create("./local/wanix-bootloader.js")
	fatal(err)
	defer f.Close()

	PackFilesTo(f)

	src, err := os.ReadFile("./dev/bootloader.js")
	fatal(err)
	f.Write(src)
}
