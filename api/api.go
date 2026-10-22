package api

import (
	"fmt"
	"log"

	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/wanix"
)

type syscaller struct {
	task *wanix.Task
}

// guard recovers panics in syscall handlers (malformed calls, type
// mismatches) and returns them as errors instead of crashing the kernel.
func guard(h rpc.HandlerFunc) rpc.HandlerFunc {
	return func(r rpc.Responder, c *rpc.Call) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("api: recovered panic in %s: %v", c.Selector(), rec)
				_ = r.Return(fmt.Errorf("internal error: %v", rec))
			}
		}()
		h(r, c)
	}
}

func Responder(sess mux.Session, task *wanix.Task) {
	syscaller := &syscaller{
		task: task,
	}

	peer := talk.NewPeer(sess, codec.CBORCodec{})
	handlers := map[string]rpc.HandlerFunc{
		"Open":       syscaller.open,
		"OpenFile":   syscaller.openFile,
		"Create":     syscaller.create,
		"Close":      syscaller.close,
		"Sync":       syscaller.sync,
		"Read":       syscaller.read,
		"Write":      syscaller.write,
		"WriteAt":    syscaller.writeAt,
		"ReadDir":    syscaller.readDir,
		"Mkdir":      syscaller.mkdir,
		"MkdirAll":   syscaller.mkdirAll,
		"Bind":       syscaller.bind,
		"Unbind":     syscaller.unbind,
		"Stat":       syscaller.stat,
		"Truncate":   syscaller.truncate,
		"WaitFor":    syscaller.waitFor,
		"Rename":     syscaller.rename,
		"Copy":       syscaller.copy,
		"Remove":     syscaller.remove,
		"RemoveAll":  syscaller.removeAll,
		"ReadFile":   syscaller.readFile,
		"WriteFile":  syscaller.writeFile,
		"AppendFile": syscaller.appendFile,
		"Fstat":      syscaller.fstat,
		"Lstat":      syscaller.lstat,
		"Chmod":      syscaller.chmod,
		"Chown":      syscaller.chown,
		"Fchmod":     syscaller.fchmod,
		"Fchown":     syscaller.fchown,
		"Ftruncate":  syscaller.ftruncate,
		"Readlink":   syscaller.readlink,
		"Symlink":    syscaller.symlink,
		"Chtimes":    syscaller.chtimes,
	}
	for selector, handler := range handlers {
		peer.Handle(selector, guard(handler))
	}
	peer.Respond()
}
