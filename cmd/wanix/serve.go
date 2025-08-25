//go:build !js && !wasm

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hugelgupf/p9/p9"
	"github.com/progrium/go-netstack/vnet"
	"tractor.dev/wanix/fs/localfs"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/external/linux"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/p9kit"
	"tractor.dev/wanix/runtime/assets"
	"tractor.dev/wanix/shell"
)

func serveCmd() *cli.Command {
	var (
		listenAddr string
		bundle     string
	)
	cmd := &cli.Command{
		Usage: "serve [dir]",
		Short: "serve directory contents with wanix overlay",
		Run: func(ctx *cli.Context, args []string) {
			var err error
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			dir, err = filepath.Abs(dir)
			if err != nil {
				log.Fatal(err)
			}
			dirfs, err := localfs.New(dir)
			if err != nil {
				log.Fatal(err)
			}

			log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

			h, p, _ := net.SplitHostPort(listenAddr)
			if h == "" {
				h = "localhost"
			}
			fmt.Printf("Serving %s files with Wanix overlay ...\n", dir)
			if bundle != "" {
				fmt.Printf("Bundle available at: http://%s:%s/?bundle=%s\n", h, p, bundle)
			} else {
				fmt.Printf("Bundle available at: http://%s:%s\n", h, p)
			}

			extra := fskit.MapFS{
				"v86":   v86.Dir,
				"linux": linux.Dir,
				"shell": shell.Dir,
			}
			fsys := fskit.UnionFS{assets.Dir, extra, dirfs}

			vn, err := vnet.New(&vnet.Configuration{
				Debug:             false,
				MTU:               1500,
				Subnet:            "192.168.127.0/24",
				GatewayIP:         "192.168.127.1",
				GatewayMacAddress: "5a:94:ef:e4:0c:dd",
				GatewayVirtualIPs: []string{},
			})
			if err != nil {
				log.Fatal(err)
			}

			http.Handle("/.well-known/", http.NotFoundHandler())
			http.Handle("/.well-known/ethernet", ethernetHandler(vn))

			p9srv := p9.NewServer(p9kit.Attacher(dirfs, p9kit.WithXattrAttrStore()))
			http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if websocket.IsWebSocketUpgrade(r) {
					p9Handler(p9srv, w, r)
					return
				}

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
	cmd.Flags().StringVar(&bundle, "bundle", "", "default bundle to use")
	return cmd
}

func p9Handler(srv *p9.Server, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer ws.Close()

	log.Println("p9 session started")
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	go func() {
		for {
			typ, buf, err := ws.ReadMessage()
			if err != nil {
				log.Println("ws->9p:", err)
				break
			}
			if typ != websocket.BinaryMessage {
				continue
			}
			if _, err := inW.Write(buf); err != nil {
				log.Println("ws->9p:", err)
				break
			}
		}
	}()

	go func() {
		for {
			// Read message length (4 bytes)
			sizeBuf := make([]byte, 4)
			_, err := io.ReadFull(outR, sizeBuf)
			if err != nil {
				log.Println("9p->ws:", err)
				break
			}
			messageSize := int(sizeBuf[3])<<24 | int(sizeBuf[2])<<16 | int(sizeBuf[1])<<8 | int(sizeBuf[0])
			payloadSize := messageSize - 4

			messageBuf := make([]byte, payloadSize)
			_, err = io.ReadFull(outR, messageBuf)
			if err != nil {
				log.Println("9p->ws:", err)
				break
			}

			buf := append(sizeBuf, messageBuf...)
			if err := ws.WriteMessage(websocket.BinaryMessage, buf); err != nil {
				log.Println("9p->ws:", err)
				break
			}
		}
	}()

	if err := srv.Handle(inR, outW); err != nil {
		log.Println("9p session ended:", err)
	}
}

func ethernetHandler(vn *vnet.VirtualNetwork) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if vn == nil {
			http.Error(w, "ethernet not available", http.StatusNotFound)
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

		log.Println("ethernet session started")

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

// qemuAdapter wraps a websocket connection and converts
// messages to qemu length prefixed protocol and back
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
