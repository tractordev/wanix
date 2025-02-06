//go:build js && wasm

package main

import (
	"log"
	"syscall/js"

	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/jsutil"
)

func main() {
	k := kernel.New()
	k.AddModule("#web", web.New(k.Fsys))

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	root.Bind("#fsys", "fsys")
	root.Bind("#proc", "proc")
	root.Bind("#web", "web")

	fs.ReadFile(root.Namespace(), "fsys/new/opfs")
	fs.WriteFile(root.Namespace(), "fsys/1/ctl", []byte("mount"), 0755)
	fs.WriteFile(root.Namespace(), "proc/1/ctl", []byte("bind fsys/1/mount opfs"), 0755)

	port := js.Global().Get("window").Get("wanixPort")
	wr := &jsutil.Writer{Value: port}
	rd := &jsutil.Reader{Value: port}
	sess, err := mux.DialIO(wr, rd)
	if err != nil {
		log.Fatal(err)
	}

	peer := talk.NewPeer(sess, codec.CBORCodec{})
	setupAPI(peer, root)
	peer.Respond()
}
