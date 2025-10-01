//go:build js && wasm

package runtime

import "syscall/js"

var instance js.Value

func Instance() js.Value {
	if instance.IsUndefined() {
		instance = js.Global().Get("__wanix").Get("pending").Call("pop")
		if instance.IsUndefined() {
			panic("no wanix instance found")
		}
	}
	return instance
}
