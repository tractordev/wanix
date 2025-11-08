//go:build js && wasm

package worker

import (
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"strings"
	"syscall/js"
)

//go:embed lib.js worker.js
var Dir embed.FS

func BlobURL() string {
	lib, err := fs.ReadFile(Dir, "lib.js")
	if err != nil {
		panic(err)
	}
	worker, err := fs.ReadFile(Dir, "worker.js")
	if err != nil {
		panic(err)
	}
	libImport := fmt.Sprintf("data:text/javascript;base64,%s", base64.StdEncoding.EncodeToString(lib))
	workerSrc := strings.Replace(string(worker), "./lib.js", libImport, 1)
	blob := js.Global().Get("Blob").New(js.ValueOf([]any{workerSrc}), js.ValueOf(map[string]any{"type": "text/javascript"}))
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	return url.String()
}
