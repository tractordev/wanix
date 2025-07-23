//go:build !js && !wasm

package main

import (
	"log"

	"tractor.dev/toolkit-go/engine"
	"tractor.dev/toolkit-go/engine/cli"
)

var Version string

func main() {
	engine.Run(Main{})
}

type Main struct{}

func (m *Main) InitializeCLI(root *cli.Command) {
	root.Usage = "wanix"
	root.Version = Version
	root.AddCommand(serveCmd())
	root.AddCommand(exportCmd())

}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
