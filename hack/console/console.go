//go:build !js && !wasm && console

package console

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"golang.org/x/net/websocket"
	"golang.org/x/term"
	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/external/linux"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/runtime/assets"
)

func AddConsoleCmd(root *cli.Command) {
	cmd := &cli.Command{
		Usage: "console",
		Short: "enter wanix console",
		Run: func(ctx *cli.Context, args []string) {

			l, err := net.Listen("tcp", "localhost:0")
			fatal(err)

			defer l.Close()

			fsys := fskit.UnionFS{assets.Dir, fskit.MapFS{
				"v86":   v86.Dir,
				"linux": linux.Dir,
			}}

			http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Cross-Origin-Opener-Policy", "same-origin")
				w.Header().Add("Cross-Origin-Embedder-Policy", "require-corp")
				http.FileServerFS(fsys).ServeHTTP(w, r)
			}))
			http.Handle("/.tty", websocket.Handler(func(conn *websocket.Conn) {
				conn.PayloadType = websocket.BinaryFrame

				oldstate, err := term.MakeRaw(int(os.Stdin.Fd()))
				fatal(err)

				defer term.Restore(int(os.Stdin.Fd()), oldstate)

				go func() {
					if _, err := io.Copy(os.Stdout, conn); err != nil {
						log.Println(err)
					}
				}()

				buffer := make([]byte, 1024)
				for {
					n, err := os.Stdin.Read(buffer)
					if err != nil {
						term.Restore(int(os.Stdin.Fd()), oldstate)
						log.Fatal("Error reading from stdin:", err)
					}

					for i := 0; i < n; i++ {
						// Check for Ctrl-D (ASCII 4)
						if buffer[i] == 4 {
							term.Restore(int(os.Stdin.Fd()), oldstate)
							conn.Close()
							fmt.Println("Ctrl-D detected")
							return
						}
					}

					//processed := bytes.ReplaceAll(buffer[:n], []byte{'\n'}, []byte{'\r', '\n'})
					_, err = conn.Write(buffer[:n])
					if err != nil {
						log.Println(err)
					}
				}

			}))

			hostname := fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port)
			url := fmt.Sprintf("http://%s/?tty=ws://%s/.tty", hostname, hostname)

			log.Printf("serving at %s", url)
			fatal(http.Serve(l, nil))

		},
	}
	root.AddCommand(cmd)
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
