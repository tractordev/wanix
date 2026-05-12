//go:build js && wasm

package gojs

import (
	"log"
	"os"
	"syscall/js"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/misc/jsutil"
)

func Export(fsys fs.FS, debug bool) error {
	msgch := js.Global().Get("MessageChannel").New()
	js.Global().Get("self").Call("postMessage", map[string]any{
		"export": msgch.Get("port2"),
	}, []any{msgch.Get("port2")})

	var o []p9.ServerOpt
	if debug {
		o = append(o, p9.WithServerLogger(log.New(os.Stderr, "", log.LstdFlags)))
	}
	rwc := jsutil.NewPortReadWriter(msgch.Get("port1"))
	srv := p9.NewServer(p9kit.Attacher(fsys, p9kit.WithMemAttrStore()), o...)
	go func() {
		if err := srv.Handle(rwc, rwc); err != nil {
			log.Fatal(err)
		}
	}()
	msgch.Get("port1").Call("postMessage", "!") // signal to worker that export is ready
	return nil
}
