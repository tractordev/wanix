package web

import (
	"syscall/js"
)

type UI struct{}

func (s *UI) InitializeJS() {
	js.Global().Get("sys").Call("call", "host.loadStylesheet", []any{"/~dev/kernel/web/ui/style.css"})
	js.Global().Get("sys").Call("call", "host.loadApp", []any{"terminal", "/~dev/internal/app/terminal/index.html", true})
}
