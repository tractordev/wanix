//go:build js && wasm

package sys

import (
	"log"
	"strconv"
	"syscall/js"
)

//go:wasmimport wanix getInstanceID
func getInstanceID() int32

func Element() js.Value {
	id := int(getInstanceID())
	element := js.Global().Get("__wanix").Get(strconv.Itoa(id))
	if element.IsUndefined() {
		log.Panicf("no wanix element registered for id %d", id)
	}
	return element
}
