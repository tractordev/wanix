package jazz

import (
	"fmt"
	"os"
	"syscall/js"
)

// placeholder for jazz cmd moved in from shell jazz builtin
func main() {
	args := os.Args
	if len(args) == 0 {
		return
	}
	// if args[0] == "mount" {
	// 	newfs := fs.NewJazzFs()
	// 	fs.Reset(newfs)
	// 	return
	// }
	var jsargs []any
	if len(args) > 1 {
		for _, arg := range args[1:] {
			jsargs = append(jsargs, arg)
		}
	}
	ret := js.Global().Get("wanix").Get("jazzfs").Get(args[0]).Invoke(jsargs...)
	if !ret.IsUndefined() {
		fmt.Fprintf(t, "%s\n", js.Global().Get("JSON").Call("stringify", ret))
	}
}
