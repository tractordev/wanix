//go:build !js && !wasm

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/hugelgupf/p9/p9"
	"github.com/progrium/go-netstack/vnet"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs/net/ether"
	"tractor.dev/wanix/fs/p9kit"
)

func hostCmd() *cli.Command {
	cmd := &cli.Command{
		Hidden: true,
		Usage:  "host",
		Short:  "native host subcommands",
	}
	cmd.AddCommand(hostNetfsCmd())
	return cmd
}

func hostNetfsCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "netfs",
		Short: "wip",
		Run: func(ctx *cli.Context, args []string) {
			cfg := &vnet.Configuration{
				Debug:             false,
				MTU:               1500,
				Subnet:            "10.0.0.0/8",
				GatewayIP:         "10.0.0.1",
				GatewayMacAddress: "5a:94:ef:e4:0c:dd",
				GatewayVirtualIPs: []string{},
			}
			vn, err := vnet.New(cfg)
			if err != nil {
				log.Fatal(err)
			}
			etherfs := ether.New(vn, cfg)
			p9srv := p9.NewServer(p9kit.Attacher(etherfs))

			http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p9Handler(p9srv, w, r)
			}))
			fmt.Println("Listening on :9090")
			http.ListenAndServe(":9090", nil)
		},
	}
	return cmd
}
