//go:build js && wasm

package xterm

import (
	"embed"
	"fmt"
	"sync"
	"syscall/js"

	"tractor.dev/wanix/web/jsutil"
)

//go:embed *.css *.js
var assets embed.FS

func assetBlob(name, typ string) (js.Value, error) {
	asset, err := assets.ReadFile(name)
	if err != nil {
		return js.Value{}, err
	}
	jsBuf := js.Global().Get("Uint8Array").New(len(asset))
	js.CopyBytesToJS(jsBuf, asset)
	return js.Global().Get("Blob").New([]any{
		jsBuf,
		fmt.Sprintf("\n//# sourceURL=embedded/%s\n", name),
	}, map[string]any{
		"type": typ,
	}), nil
}

var once sync.Once

func Load() {
	once.Do(func() {
		xtermCSS, err := assetBlob("xterm-5.3.0.min.css", "text/css")
		if err != nil {
			panic(err)
		}
		jsutil.LoadStylesheet(js.Global().Get("URL").Call("createObjectURL", xtermCSS).String())

		fitJS, err := assetBlob("xterm-fit-0.8.0.min.js", "text/javascript")
		if err != nil {
			panic(err)
		}
		jsutil.LoadScript(js.Global().Get("URL").Call("createObjectURL", fitJS).String(), false)

		xtermJS, err := assetBlob("xterm-5.3.0.min.js", "text/javascript")
		if err != nil {
			panic(err)
		}
		promise := jsutil.LoadScript(js.Global().Get("URL").Call("createObjectURL", xtermJS).String(), false)
		_, err = jsutil.AwaitErr(promise)
		if err != nil {
			panic(err)
		}
	})
}
