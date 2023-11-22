package tty

import (
	"io"
	"syscall/js"

	"tractor.dev/wanix/internal/jsutil"
)

func (s *Service) InitializeJS() {
	// expose to tasks
	js.Global().Get("api").Set("tty", map[string]any{
		"open": map[string]any{
			"respondRPC": js.FuncOf(s.open),
		},
	})
	// expose to task host
	js.Global().Get("sys").Call("handle", "tty.open", map[string]any{
		"respondRPC": js.FuncOf(s.open),
	})
}

func (s *Service) open(this js.Value, jsArgs []js.Value) any {
	var (
		responder = jsArgs[0]
		call      = jsArgs[1]
	)
	return jsutil.Promise(func() (any, error) {
		params := jsutil.Await(call.Call("receive"))
		var (
			path = params.Index(0).String()
			args = jsutil.ToGoStringSlice(params.Index(1))
			// todo: env, dir, rows, cols, term
		)

		p, tty, err := s.Open(path, args)
		if err != nil {
			responder.Call("return", jsutil.ToJSError(err))
			return nil, err
		}
		ch := jsutil.Await(responder.Call("continue"))
		go func() {
			io.Copy(&jsutil.Writer{ch}, tty)
		}()
		io.Copy(tty, &jsutil.Reader{ch}) // stdin blocks close
		ch.Call("close")
		tty.Close()
		p.Terminate()
		return nil, nil
	})
}
