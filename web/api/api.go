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
	fds       map[int]*openFile
	fdCounter int
}

type openFile struct {
	file fs.File
	path string
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
		fds:       make(map[int]*openFile),
		fdCounter: 3, // 0-2 reserved for stdio
	}

	peer := talk.NewPeer(sess, codec.CBORCodec{})
	peer.Handle("Open", rpc.HandlerFunc(syscaller.open))
	peer.Handle("OpenFile", rpc.HandlerFunc(syscaller.openFile))
	peer.Handle("Create", rpc.HandlerFunc(syscaller.create))
	peer.Handle("Close", rpc.HandlerFunc(syscaller.close))
	peer.Handle("Sync", rpc.HandlerFunc(syscaller.sync))
	peer.Handle("Read", rpc.HandlerFunc(syscaller.read))
	peer.Handle("Write", rpc.HandlerFunc(syscaller.write))
	peer.Handle("WriteAt", rpc.HandlerFunc(syscaller.writeAt))
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
	peer.Handle("Fstat", rpc.HandlerFunc(syscaller.fstat))
	peer.Handle("Lstat", rpc.HandlerFunc(syscaller.lstat))
	peer.Handle("Chmod", rpc.HandlerFunc(syscaller.chmod))
	peer.Handle("Chown", rpc.HandlerFunc(syscaller.chown))
	peer.Handle("Fchmod", rpc.HandlerFunc(syscaller.fchmod))
	peer.Handle("Fchown", rpc.HandlerFunc(syscaller.fchown))
	peer.Handle("Ftruncate", rpc.HandlerFunc(syscaller.ftruncate))
	peer.Handle("Readlink", rpc.HandlerFunc(syscaller.readlink))
	peer.Handle("Symlink", rpc.HandlerFunc(syscaller.symlink))
	peer.Handle("Chtimes", rpc.HandlerFunc(syscaller.chtimes))
	peer.Respond()
}
