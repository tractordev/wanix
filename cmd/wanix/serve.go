//go:build !js && !wasm

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/progrium/go-netstack/vnet"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/external/linux"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/runtime/assets"
	"tractor.dev/wanix/shell"
)

func serveCmd() *cli.Command {
	var (
		listenAddr string
	)
	cmd := &cli.Command{
		Usage: "serve [dir]",
		Short: "serve directory contents with wanix overlay",
		Run: func(ctx *cli.Context, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			dirFS := os.DirFS(dir)

			log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

			h, p, _ := net.SplitHostPort(listenAddr)
			if h == "" {
				h = "localhost"
			}
			fmt.Printf("serving %s files with wanix overlay on http://%s:%s ...\n", dir, h, p)

			extra := fskit.MapFS{
				"v86":   v86.Dir,
				"linux": linux.Dir,
				"shell": shell.Dir,
			}
			fsys := fskit.UnionFS{assets.Dir, extra, dirFS}

			go serveNetwork()

			http.Handle("/.well-known/", http.NotFoundHandler())

			http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
				w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")

				if r.URL.Path == "/wanix.wasm" {
					w.Header().Add("Content-Type", "application/wasm")
					// TODO: a flag to prefer variant
					wasmFsys, err := assets.WasmFS(false)
					if err != nil {
						log.Fatal(err)
					}
					http.ServeFileFS(w, r, wasmFsys, "wanix.wasm")
					return
				}

				http.FileServerFS(fsys).ServeHTTP(w, r)
			}))
			http.ListenAndServe(listenAddr, nil)
		},
	}
	cmd.Flags().StringVar(&listenAddr, "listen", ":7654", "addr to serve on")
	return cmd
}

func serveNetwork() {
	vn, err := vnet.New(&vnet.Configuration{
		Debug:             false,
		MTU:               1500,
		Subnet:            "192.168.127.0/24",
		GatewayIP:         "192.168.127.1",
		GatewayMacAddress: "5a:94:ef:e4:0c:dd",
		GatewayVirtualIPs: []string{"192.168.127.253"},
	})
	if err != nil {
		log.Fatal(err)
	}

	addr := ":8777"
	if os.Getenv("NET_LISTEN") != "" {
		addr = os.Getenv("NET_LISTEN")
	}
	if strings.HasPrefix(addr, ":") {
		addr = "0.0.0.0" + addr
	}
	if err := http.ListenAndServe(addr, handler(vn)); err != nil {
		log.Fatal(err)
	}
}

func handler(vn *vnet.VirtualNetwork) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vn == nil {
			http.Error(w, "network not available", http.StatusNotFound)
			return
		}
		if !websocket.IsWebSocketUpgrade(r) {
			http.Error(w, "expecting websocket upgrade", http.StatusBadRequest)
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}
		defer ws.Close()

		fmt.Println("network session started")

		if err := vn.AcceptQemu(r.Context(), &qemuAdapter{Conn: ws}); err != nil {
			if strings.Contains(err.Error(), "websocket: close") {
				return
			}
			log.Println(err)
			return
		}
	})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type qemuAdapter struct {
	*websocket.Conn
	mu          sync.Mutex
	readBuffer  []byte
	writeBuffer []byte
	readOffset  int
}

func (q *qemuAdapter) Read(p []byte) (n int, err error) {
	if len(q.readBuffer) == 0 {
		_, message, err := q.ReadMessage()
		if err != nil {
			return 0, err
		}
		length := uint32(len(message))
		lengthPrefix := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthPrefix, length)
		q.readBuffer = append(lengthPrefix, message...)
		q.readOffset = 0
	}

	n = copy(p, q.readBuffer[q.readOffset:])
	q.readOffset += n
	if q.readOffset >= len(q.readBuffer) {
		q.readBuffer = nil
	}
	return n, nil
}

func (q *qemuAdapter) Write(p []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.writeBuffer = append(q.writeBuffer, p...)

	if len(q.writeBuffer) < 4 {
		return len(p), nil
	}

	length := binary.BigEndian.Uint32(q.writeBuffer[:4])
	if len(q.writeBuffer) < int(length)+4 {
		return len(p), nil
	}

	err := q.WriteMessage(websocket.BinaryMessage, q.writeBuffer[4:4+length])
	if err != nil {
		return 0, err
	}

	q.writeBuffer = q.writeBuffer[4+length:]
	return len(p), nil
}

func (c *qemuAdapter) LocalAddr() net.Addr {
	return &net.UnixAddr{}
}

func (c *qemuAdapter) RemoteAddr() net.Addr {
	return &net.UnixAddr{}
}

func (c *qemuAdapter) SetDeadline(t time.Time) error {
	return nil
}
func (c *qemuAdapter) SetReadDeadline(t time.Time) error {
	return nil
}
func (c *qemuAdapter) SetWriteDeadline(t time.Time) error {
	return nil
}
