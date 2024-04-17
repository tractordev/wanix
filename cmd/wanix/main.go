package main

import (
	"log"

	"tractor.dev/toolkit-go/engine"
	"tractor.dev/toolkit-go/engine/cli"
)

func main() {
	engine.Run(Main{})
}

type Main struct{}

func (m *Main) InitializeCLI(root *cli.Command) {
	root.Usage = "wanix"
	root.AddCommand(devCmd())
	root.AddCommand(loaderCmd())
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
