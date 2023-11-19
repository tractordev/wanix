package web

import (
	"syscall/js"
)

type UI struct{}

func (s *UI) InitializeJS() {
	js.Global().Get("sys").Call("call", "host.loadStylesheet", []any{"/sys/dev/kernel/web/ui/style.css"})
	js.Global().Get("sys").Call("call", "host.loadApp", []any{"terminal", "/sys/dev/internal/app/terminal", true})
}
