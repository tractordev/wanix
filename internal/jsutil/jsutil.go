package jsutil

import (
	"errors"
	"io"
	"io/fs"
	"syscall"
	"syscall/js"
)

// Returns the syscall `response.value`.
// To access the response itself see `WanixSyscallResp` instead.
func WanixSyscall(fn string, args ...any) (js.Value, error) {
	response, err := WanixSyscallResp(fn, args...)
	if err != nil {
		return js.Null(), err
	}
	return response.Get("value"), nil
}

// Useful for syscalls involving streams (i.e. stdio).
func WanixSyscallResp(fn string, args ...any) (response js.Value, err error) {
	return AwaitErr(js.Global().Get("sys").Call("call", fn, args))
}

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
		return js.Undefined(), js.Error{rejected}
	}
	return resolved, nil
}

func Promise(fn func() (any, error)) any {
	return js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, args []js.Value) any {
		resolve := args[0]
		reject := args[1]
		go func() {
			v, err := fn()
			if err != nil {
				reject.Invoke(err.Error())
				return
			}
			resolve.Invoke(v)
		}()
		return nil
	}))
}

func Log(args ...any) {
	js.Global().Get("console").Call("log", args...)
}

func Err(args ...any) {
	js.Global().Get("console").Call("error", args...)
}

func HasProp(jsObj js.Value, prop string) bool {
	return js.Global().Get("Object").Call("hasOwn", jsObj, prop).Bool()
}

func CopyObj(jsObj js.Value) js.Value {
	return js.Global().Get("Object").Call("assign", map[string]any{}, jsObj)
}

func ToJSError(err error) js.Value {
	jsErr := js.Global().Get("Error").New(err.Error())
	if sysErr, ok := err.(syscall.Errno); ok {
		jsErr.Set("code", errnoString(sysErr))
	}
	// I guess fs errors arent syscall errors
	if errors.Is(err, fs.ErrClosed) {
		jsErr.Set("code", "EIO") // not sure on this one
	}
	if errors.Is(err, fs.ErrExist) {
		jsErr.Set("code", "EEXIST")
	}
	if errors.Is(err, fs.ErrInvalid) {
		jsErr.Set("code", "EINVAL")
	}
	if errors.Is(err, fs.ErrNotExist) {
		jsErr.Set("code", "ENOENT")
	}
	if errors.Is(err, fs.ErrPermission) {
		jsErr.Set("code", "EPERM")
	}
	return jsErr
}

func ToJSArray[S ~[]E, E any](s S) []any {
	r := make([]any, len(s))
	for i, e := range s {
		r[i] = e
	}
	return r
}

func ToJSMap(m map[string]string) map[string]any {
	r := make(map[string]any, len(m))
	for k, v := range m {
		r[k] = v
	}
	return r
}

func ToGoStringSlice(jsArray js.Value) []string {
	if jsArray.Type() != js.TypeObject || !jsArray.InstanceOf(js.Global().Get("Array")) {
		panic("provided js.Value is not a JavaScript array")
	}

	length := jsArray.Length()
	result := make([]string, length)

	if length == 0 {
		return result
	}

	for i := 0; i < length; i++ {
		result[i] = jsArray.Index(i).String()
	}

	return result
}

func ToGoSlice(jsArray js.Value) []any {
	if jsArray.Type() != js.TypeObject || !jsArray.InstanceOf(js.Global().Get("Array")) {
		panic("provided js.Value is not a JavaScript array")
	}

	length := jsArray.Length()
	result := make([]any, length)

	if length == 0 {
		return result
	}

	for i := 0; i < length; i++ {
		val := jsArray.Index(i)
		switch val.Type() {
		case js.TypeString:
			result[i] = val.String()
		case js.TypeBoolean:
			result[i] = val.Bool()
		case js.TypeNumber:
			result[i] = val.Int()
		default:
			panic("unsupported type for conversion")
		}
	}

	return result
}

func ToGoStringMap(jsObj js.Value) map[string]string {
	if jsObj.Type() != js.TypeObject {
		panic("provided js.Value is not a JavaScript object")
	}

	result := make(map[string]string)

	jsKeys := js.Global().Get("Object").Call("keys", jsObj)
	for i := 0; i < jsKeys.Length(); i++ {
		key := jsKeys.Index(i).String()
		result[key] = jsObj.Get(key).String()
	}

	return result
}

func ToGoMap(jsObj js.Value) map[string]any {
	if jsObj.Type() != js.TypeObject {
		panic("provided js.Value is not a JavaScript object")
	}

	result := make(map[string]any)

	jsKeys := js.Global().Get("Object").Call("keys", jsObj)
	for i := 0; i < jsKeys.Length(); i++ {
		key := jsKeys.Index(i).String()
		jsVal := jsObj.Get(key)

		switch jsVal.Type() {
		case js.TypeString:
			result[key] = jsVal.String()
		case js.TypeBoolean:
			result[key] = jsVal.Bool()
		case js.TypeNumber:
			result[key] = jsVal.Int()
		default:
			panic("unsupported type for conversion")
		}
	}

	return result
}

type Writer struct {
	js.Value
}

func (w *Writer) Write(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(buf, p)
	nn, e := AwaitErr(w.Value.Call("write", buf))
	if e != nil {
		return 0, e
	}
	n = nn.Int()
	return
}

func (w *Writer) Close() error {
	w.Value.Call("close")
	return nil
}

type Reader struct {
	js.Value
}

func (r *Reader) Read(p []byte) (n int, err error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	nn, e := AwaitErr(r.Value.Call("read", buf))
	if e != nil {
		return 0, e
	}
	js.CopyBytesToGo(p, buf)
	if nn.IsNull() {
		return 0, io.EOF
	}
	return nn.Int(), nil
}

func (r *Reader) Close() error {
	r.Value.Call("close")
	return nil
}

type ReadWriter struct {
	io.ReadCloser
	io.WriteCloser
}

func (rw *ReadWriter) Close() (err error) {
	err = rw.ReadCloser.Close()
	if err != nil {
		return
	}
	err = rw.WriteCloser.Close()
	return
}
