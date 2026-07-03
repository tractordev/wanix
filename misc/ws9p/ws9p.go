package ws9p

import (
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs/p9kit"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Handle(fsys fs.FS, w http.ResponseWriter, r *http.Request, logf func(string, ...any)) {
	if logf == nil {
		logf = func(format string, args ...any) {}
	}
	srv := p9.NewServer(p9kit.Attacher(fsys, p9kit.WithXattrAttrStore()))
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer ws.Close()

	logf("9p session started\n")
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	go func() {
		for {
			typ, buf, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Println("ws->9p:", err)
					break
				}
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
		logf("9p session ended: %v\n", err)
	}
}
