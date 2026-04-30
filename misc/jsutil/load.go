//go:build js && wasm

package jsutil

import (
	"syscall/js"
)

func LoadStylesheet(url string) {
	doc := js.Global().Get("document")
	link := doc.Call("createElement", "link")
	link.Set("href", url)
	link.Set("rel", "stylesheet")
	link.Set("type", "text/css")
	doc.Get("head").Call("appendChild", link)
}

func LoadScript(url string, module bool) js.Value {
	promise := js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, args []js.Value) any {
		resolve := args[0]
		reject := args[1]

		doc := js.Global().Get("document")
		script := doc.Call("createElement", "script")
		script.Set("src", url)
		if module {
			script.Set("type", "module")
		}
		script.Set("onload", resolve)
		script.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			err := js.Global().Get("Error").New("Failed to load script: " + url)
			reject.Invoke(err)
			return nil
		}))
		doc.Get("head").Call("appendChild", script)
		return nil
	}))
	return promise
}
