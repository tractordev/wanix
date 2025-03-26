//go:build js && wasm

package jsutil

import "syscall/js"

func Await(promise js.Value) js.Value {
	ch := make(chan js.Value, 1)
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0]
		return nil
	}))
	return <-ch
}

func AwaitErr(promise js.Value) (js.Value, error) {
	ch := make(chan js.Value, 2)
	promise.Call("then",
		js.FuncOf(func(this js.Value, args []js.Value) any {
			ch <- args[0] // resolve
			ch <- js.Undefined()
			return nil
		}),
		js.FuncOf(func(this js.Value, args []js.Value) any {
			ch <- js.Undefined()
			ch <- args[0] // reject
			return nil
		}),
	)
	resolved := <-ch
	rejected := <-ch
	if rejected.Truthy() {
		return js.Undefined(), js.Error{Value: rejected}
	}
	return resolved, nil
}
