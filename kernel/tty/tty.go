package tty

import (
	"io"
	"syscall/js"

	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/kernel/proc"
)

type Service struct {
	Proc *proc.Service
}

func (s *Service) InitializeJS() {
	// expose to subtasks
	js.Global().Get("api").Set("tty", map[string]any{
		"open": map[string]any{
			"respondRPC": js.FuncOf(s.open),
		},
	})
	// expose to host
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

		p, tty := s.Open(path, args)
		ch := jsutil.Await(responder.Call("continue"))
		go func() {
			io.Copy(&jsutil.Writer{ch}, tty)
		}()
		io.Copy(tty, &jsutil.Reader{ch}) // stdin blocks close
		ch.Call("close")
		tty.Close()
		p.Kill()
		return nil, nil
	})
}

// todo: env, dir, rows, cols, term
func (s *Service) Open(path string, args []string) (*proc.Process, io.ReadWriteCloser) {
	// rows, cols, term would just be added to env
	p := s.Proc.Spawn(path, args, nil, "")
	stdin := p.Stdin()
	output := p.Output()
	return p, &jsutil.ReadWriter{
		ReadCloser:  output,
		WriteCloser: stdin,
	}
}
