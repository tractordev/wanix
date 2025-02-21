//go:build js && wasm

package api

import (
	"fmt"
	"io"
	"log"
	"syscall/js"

	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/kernel/proc"
	"tractor.dev/wanix/web/jsutil"
)

func PortResponder(port js.Value, root *proc.Process) {
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

type openInode struct {
	Path    string
	Name    string
	Size    int64
	IsDir   bool
	Mode    uint32
	Entries []openInode
	Error   string
}

func setupAPI(peer *talk.Peer, root *proc.Process) {
	fds := make(map[int]fs.File)
	fdCounter := 0

	peer.Handle("Open", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)

		f, err := root.Namespace().Open(args[0])
		if err != nil {
			r.Return(err)
			return
		}

		fdCounter++
		fds[fdCounter] = f
		r.Return(fdCounter)
	}))
	peer.Handle("Close", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []any
		c.Receive(&args)

		fd, ok := args[0].(uint64)
		if !ok {
			log.Panicf("arg 0 is not a uint64: %T %v", args[1], args[1])
		}

		f, ok := fds[int(fd)]
		if !ok {
			r.Return(fs.ErrInvalid)
			return
		}

		r.Return(f.Close())
		delete(fds, int(fd))
	}))
	peer.Handle("Read", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []any
		c.Receive(&args)

		fd, ok := args[0].(uint64)
		if !ok {
			log.Panicf("arg 0 is not a uint64: %T %v", args[1], args[1])
		}

		f, ok := fds[int(fd)]
		if !ok {
			r.Return(fs.ErrInvalid)
			return
		}

		count, ok := args[1].(uint64)
		if !ok {
			panic("arg 1 is not a uint64")
		}

		buf := make([]byte, count)
		n, err := f.Read(buf)
		if err == io.EOF {
			r.Return([]byte{})
			return
		}
		if err != nil {
			r.Return(err)
			return
		}

		r.Return(buf[:n])
	}))
	peer.Handle("Write", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []any
		c.Receive(&args)

		fd, ok := args[0].(uint64)
		if !ok {
			log.Panicf("arg 0 is not a uint64: %T %v", args[1], args[1])
		}

		f, ok := fds[int(fd)]
		if !ok {
			r.Return(fs.ErrInvalid)
			return
		}

		data, ok := args[1].([]byte)
		if !ok {
			panic("arg 1 is not a []byte")
		}

		n, err := fs.Write(f, data)
		if err != nil {
			r.Return(err)
			return
		}

		r.Return(n)
	}))

	// peer.Handle("OpenInode", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
	// 	var args []string
	// 	c.Receive(&args)
	// 	// log.Println("OpenInode", args)

	// 	f, err := root.Namespace().Open(args[0])
	// 	if err != nil {
	// 		if errors.Is(err, fs.ErrNotExist) {
	// 			r.Return(openInode{Error: fs.ErrNotExist.Error()})
	// 			return
	// 		}
	// 		r.Return(err)
	// 		return
	// 	}
	// 	defer f.Close()

	// 	fi, err := f.Stat()
	// 	if err != nil {
	// 		r.Return(err)
	// 		return
	// 	}

	// 	var entries []openInode
	// 	if fi.IsDir() {
	// 		e, err := fs.ReadDir(root.Namespace(), args[0])
	// 		if err != nil {
	// 			r.Return(err)
	// 			return
	// 		}
	// 		for _, ee := range e {
	// 			efi, err := ee.Info()
	// 			if err != nil {
	// 				r.Return(err)
	// 				return
	// 			}
	// 			entries = append(entries, openInode{
	// 				Name:  efi.Name(),
	// 				IsDir: efi.IsDir(),
	// 				Mode:  uint32(efi.Mode()),
	// 				Size:  int64(efi.Size()),
	// 			})
	// 		}
	// 	}

	// 	node := openInode{
	// 		Path:    args[0],
	// 		Name:    path.Base(args[0]),
	// 		Size:    fi.Size(),
	// 		IsDir:   fi.IsDir(),
	// 		Mode:    uint32(fi.Mode()),
	// 		Entries: entries,
	// 	}

	// 	err = r.Return(node)
	// 	if err != nil {
	// 		log.Println("err:", err)
	// 	}
	// }))
	peer.Handle("ReadDir", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)

		// log.Println("ReadDir", args)

		dir, err := fs.ReadDir(root.Namespace(), args[0])
		if err != nil {
			log.Println("err:", args[0], err)
			r.Return(err)
			return
		}

		var entries []string
		for _, e := range dir {
			name := e.Name()
			if e.IsDir() {
				name = name + "/"
			}
			entries = append(entries, name)
		}
		r.Return(entries)
	}))
	peer.Handle("Mkdir", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)

		err := fs.MkdirAll(root.Namespace(), args[0], 0755)
		if err != nil {
			r.Return(err)
			return
		}

	}))
	peer.Handle("Stat", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)

		fi, err := fs.Stat(root.Namespace(), args[0])
		if err != nil {
			r.Return(err)
			return
		}

		r.Return(fmt.Sprintf("%v", fi))
	}))
	peer.Handle("Remove", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)

		err := fs.Remove(root.Namespace(), args[0])
		if err != nil {
			r.Return(err)
			return
		}
	}))
	peer.Handle("ReadFile", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)

		// log.Println("ReadFile", args)

		b, err := fs.ReadFile(root.Namespace(), args[0])
		if err != nil {
			r.Return(err)
			return
		}

		r.Return(b)
	}))
	peer.Handle("WriteFile", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []any
		c.Receive(&args)

		// log.Println("WriteFile", args)

		name, ok := args[0].(string)
		if !ok {
			panic("arg 0 is not a string")
		}

		data, ok := args[1].([]byte)
		if !ok {
			panic("arg 0 is not a []byte")
		}

		err := fs.WriteFile(root.Namespace(), name, data, 0x644)
		if err != nil {
			r.Return(err)
			return
		}
	}))
}
