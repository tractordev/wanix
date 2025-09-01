//go:build js && wasm

package api

import (
	"log"
	"syscall/js"

	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/task"
	"tractor.dev/wanix/web/jsutil"
)

type syscaller struct {
	task      *task.Resource
	fds       map[int]fs.File
	fdCounter int
}

func PortResponder(port js.Value, root *task.Resource) {
	wr := &jsutil.Writer{Value: port}
	rd := &jsutil.Reader{Value: port}
	sess, err := mux.DialIO(wr, rd)
	if err != nil {
		log.Fatal(err)
	}

	syscaller := &syscaller{
		task:      root,
		fds:       make(map[int]fs.File),
		fdCounter: 0,
	}

	peer := talk.NewPeer(sess, codec.CBORCodec{})
	peer.Handle("Open", rpc.HandlerFunc(syscaller.open))
	peer.Handle("Close", rpc.HandlerFunc(syscaller.close))
	peer.Handle("Sync", rpc.HandlerFunc(syscaller.sync))
	peer.Handle("Read", rpc.HandlerFunc(syscaller.read))
	peer.Handle("Write", rpc.HandlerFunc(syscaller.write))
	peer.Handle("ReadDir", rpc.HandlerFunc(syscaller.readDir))
	peer.Handle("Mkdir", rpc.HandlerFunc(syscaller.mkdir))
	peer.Handle("MkdirAll", rpc.HandlerFunc(syscaller.mkdirAll))
	peer.Handle("Bind", rpc.HandlerFunc(syscaller.bind))
	peer.Handle("Unbind", rpc.HandlerFunc(syscaller.unbind))
	peer.Handle("Stat", rpc.HandlerFunc(syscaller.stat))
	peer.Handle("Truncate", rpc.HandlerFunc(syscaller.truncate))
	peer.Handle("WaitFor", rpc.HandlerFunc(syscaller.waitFor))
	peer.Handle("Rename", rpc.HandlerFunc(syscaller.rename))
	peer.Handle("Copy", rpc.HandlerFunc(syscaller.copy))
	peer.Handle("Remove", rpc.HandlerFunc(syscaller.remove))
	peer.Handle("RemoveAll", rpc.HandlerFunc(syscaller.removeAll))
	peer.Handle("ReadFile", rpc.HandlerFunc(syscaller.readFile))
	peer.Handle("WriteFile", rpc.HandlerFunc(syscaller.writeFile))
	peer.Handle("AppendFile", rpc.HandlerFunc(syscaller.appendFile))
	peer.Respond()
}
