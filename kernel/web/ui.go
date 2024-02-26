package web

import (
	"tractor.dev/wanix/internal/jsutil"
)

type UI struct{}

func (s *UI) InitializeJS() {
	jsutil.WanixSyscall("host.loadStylesheet", "/sys/cmd/kernel/web/ui/style.css")
	jsutil.WanixSyscall("host.loadApp", "terminal", "/sys/app/terminal", true)
}
