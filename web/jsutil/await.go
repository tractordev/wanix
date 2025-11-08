//go:build js && wasm

package jsutil

import "syscall/js"

func Await(promise js.Value) js.Value {
	ch := make(chan js.Value, 1)
	resolveFn := js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0]
		return nil
	})
	defer resolveFn.Release()
	
	promise.Call("then", resolveFn)
	return <-ch
}

func AwaitErr(promise js.Value) (js.Value, error) {
	ch := make(chan js.Value, 2)
	
	// Create closures that will be released after promise settles
	resolveFn := js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- args[0] // resolve
		ch <- js.Undefined()
		return nil
	})
	rejectFn := js.FuncOf(func(this js.Value, args []js.Value) any {
		ch <- js.Undefined()
		ch <- args[0] // reject
		return nil
	})
	
	// Release after promise settles
	defer resolveFn.Release()
	defer rejectFn.Release()
	
	promise.Call("then", resolveFn, rejectFn)
	
	resolved := <-ch
	rejected := <-ch
	if rejected.Truthy() {
		return js.Undefined(), js.Error{Value: rejected}
	}
	return resolved, nil
}
