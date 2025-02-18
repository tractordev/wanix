//go:build js && wasm

package main

import (
	"io"
	"log"
	"syscall/js"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/kernel"
	"tractor.dev/wanix/web"
	"tractor.dev/wanix/web/api"
)

func main() {
	ctx := js.Global().Get("wanix")
	if ctx.IsUndefined() {
		log.Fatal("Wanix not initialized on this page")
	}

	k := kernel.New()
	k.AddModule("#web", web.New(k, ctx))

	root, err := k.NewRoot()
	if err != nil {
		log.Fatal(err)
	}

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	var virtioSend js.Value
	ctx.Set("virtioHandle", js.FuncOf(func(this js.Value, args []js.Value) any {
		virtioSend = args[1]
		go func() {
			buf := make([]byte, args[0].Get("byteLength").Int())
			js.CopyBytesToGo(buf, args[0])
			_, err := inW.Write(buf)
			if err != nil {
				log.Println(err)
			}
			log.Printf("virtio >>%s %v\n", p9kit.MessageTypes[int(buf[4])], buf)
		}()
		return nil
	}))

	go func() {
		for {
			// Read message length (4 bytes)
			sizeBuf := make([]byte, 4)
			_, err := io.ReadFull(outR, sizeBuf)
			if err != nil {
				log.Println("outR read size error:", err)
				break
			}
			messageSize := int(sizeBuf[3])<<24 | int(sizeBuf[2])<<16 | int(sizeBuf[1])<<8 | int(sizeBuf[0])

			// Subtract 4 bytes from messageSize since we've already read the size field
			payloadSize := messageSize - 4

			// Read the remaining message payload
			messageBuf := make([]byte, payloadSize)
			_, err = io.ReadFull(outR, messageBuf)
			if err != nil {
				log.Println("outR read message error:", err)
				break
			}

			// Combine size field and payload for complete message
			buf := append(sizeBuf, messageBuf...)

			jsBuf := js.Global().Get("Uint8Array").New(len(buf))
			js.CopyBytesToJS(jsBuf, buf)
			if virtioSend.IsUndefined() {
				log.Println("virtioSend is undefined")
				return
			}
			log.Printf("virtio <<%s %v\n", p9kit.MessageTypes[int(buf[4])], buf)
			virtioSend.Invoke(jsBuf)
		}
	}()
	srv := p9.NewServer(p9kit.Attacher(root.Namespace()))
	go func() {
		if err := srv.Handle(inR, outW); err != nil {
			log.Fatal(err)
		}
	}()

	root.Bind("#fsys", "fsys")
	root.Bind("#proc", "proc")
	root.Bind("#web", "web")

	fs.ReadFile(root.Namespace(), "fsys/new/opfs")
	fs.WriteFile(root.Namespace(), "fsys/1/ctl", []byte("mount"), 0755)
	fs.WriteFile(root.Namespace(), "proc/1/ctl", []byte("bind fsys/1/mount opfs"), 0755)

	api.PortResponder(ctx.Get("sys"), root)
}
