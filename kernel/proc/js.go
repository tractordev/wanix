package proc

import (
	"io"
	"syscall/js"

	"tractor.dev/wanix/internal/jsutil"
)

func (s *Service) InitializeJS() {
	// expose to tasks
	js.Global().Get("api").Set("proc", map[string]any{
		"spawn": js.FuncOf(s.spawn),
		"wait":  js.FuncOf(s.wait),
		"stdin": map[string]any{
			"respondRPC": js.FuncOf(s.stdin),
		},
		"stdout": map[string]any{
			"respondRPC": js.FuncOf(s.stdout),
		},
		"stderr": map[string]any{
			"respondRPC": js.FuncOf(s.stderr),
		},
	})
}

func (s *Service) spawn(this js.Value, args []js.Value) any {
	var (
		path  = args[0].String()
		args_ = jsutil.ToGoStringSlice(args[1])
		env   = jsutil.ToGoStringMap(args[2])
		dir   = args[3].String()
	)
	return jsutil.Promise(func() (any, error) {
		p, err := s.Spawn(path, args_, env, dir)
		if err != nil {
			jsutil.Err(err)
			return nil, err
		}
		return p.PID, nil
	})
}

func (s *Service) wait(this js.Value, args []js.Value) any {
	var (
		pid = args[0].Int()
	)
	return jsutil.Promise(func() (any, error) {
		p, err := s.Get(pid)
		if err != nil {
			jsutil.Err(err)
			return nil, err
		}
		return p.Wait()
	})
}

func (s *Service) stdout(this js.Value, jsArgs []js.Value) any {
	var (
		responder = jsArgs[0]
		call      = jsArgs[1]
	)
	return jsutil.Promise(func() (any, error) {
		params := jsutil.Await(call.Call("receive"))
		var (
			pid = params.Index(0).Int()
		)

		p, err := s.Get(pid)
		if err != nil {
			responder.Call("return", jsutil.ToJSError(err))
			return nil, err
		}

		rc := p.Stdout()

		ch := jsutil.Await(responder.Call("continue"))

		io.Copy(&jsutil.Writer{ch}, rc)

		ch.Call("close")
		rc.Close()
		return nil, nil
	})
}

func (s *Service) stderr(this js.Value, jsArgs []js.Value) any {
	var (
		responder = jsArgs[0]
		call      = jsArgs[1]
	)
	return jsutil.Promise(func() (any, error) {
		params := jsutil.Await(call.Call("receive"))
		var (
			pid = params.Index(0).Int()
		)

		p, err := s.Get(pid)
		if err != nil {
			responder.Call("return", jsutil.ToJSError(err))
			return nil, err
		}

		rc := p.Stderr()

		ch := jsutil.Await(responder.Call("continue"))

		io.Copy(&jsutil.Writer{ch}, rc)

		ch.Call("close")
		rc.Close()
		return nil, nil
	})
}

func (s *Service) stdin(this js.Value, jsArgs []js.Value) any {
	var (
		responder = jsArgs[0]
		call      = jsArgs[1]
	)
	return jsutil.Promise(func() (any, error) {
		params := jsutil.Await(call.Call("receive"))
		var (
			pid = params.Index(0).Int()
		)

		p, err := s.Get(pid)
		if err != nil {
			responder.Call("return", jsutil.ToJSError(err))
			return nil, err
		}

		wc := p.Stdin()

		ch := jsutil.Await(responder.Call("continue"))

		io.Copy(wc, &jsutil.Reader{ch})

		ch.Call("close")
		wc.Close()
		return nil, nil
	})
}
