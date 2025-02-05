//go:build js && wasm

package main

import (
	"errors"
	"io"
	"log"
	"path"

	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/kernel/proc"
)

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
	peer.Handle("OpenInode", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)
		// log.Println("OpenInode", args)

		f, err := root.Namespace().Open(args[0])
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				r.Return(openInode{Error: fs.ErrNotExist.Error()})
				return
			}
			r.Return(err)
			return
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			r.Return(err)
			return
		}

		var entries []openInode
		if fi.IsDir() {
			e, err := fs.ReadDir(root.Namespace(), args[0])
			if err != nil {
				r.Return(err)
				return
			}
			for _, ee := range e {
				efi, err := ee.Info()
				if err != nil {
					r.Return(err)
					return
				}
				entries = append(entries, openInode{
					Name:  efi.Name(),
					IsDir: efi.IsDir(),
					Mode:  uint32(efi.Mode()),
					Size:  int64(efi.Size()),
				})
			}
		}

		node := openInode{
			Path:    args[0],
			Name:    path.Base(args[0]),
			Size:    fi.Size(),
			IsDir:   fi.IsDir(),
			Mode:    uint32(fi.Mode()),
			Entries: entries,
		}

		err = r.Return(node)
		if err != nil {
			log.Println("err:", err)
		}
	}))
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
	peer.Handle("Remove", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []string
		c.Receive(&args)

		err := fs.Remove(root.Namespace(), args[0])
		if err != nil {
			r.Return(err)
			return
		}
	}))
	peer.Handle("Read", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []any
		c.Receive(&args)

		name, ok := args[0].(string)
		if !ok {
			panic("arg 0 is not a string")
		}

		offset, ok := args[1].(uint64)
		if !ok {
			log.Panicf("arg 1 is not a uint64: %T %v", args[1], args[1])
		}

		count, ok := args[2].(uint64)
		if !ok {
			panic("arg 2 is not a uint64")
		}

		f, err := root.Namespace().Open(name)
		if err != nil {
			r.Return(err)
			return
		}
		defer f.Close()

		buf := make([]byte, count)
		n, err := fs.ReadAt(f, buf, int64(offset))
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
