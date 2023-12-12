package web

import (
	"tractor.dev/wanix/internal/jsutil"
)

type UI struct{}

func (s *UI) InitializeJS() {
	jsutil.WanixSyscall("host.loadStylesheet", "/sys/dev/kernel/web/ui/style.css")
	jsutil.WanixSyscall("host.loadApp", "terminal", "/sys/dev/internal/app/terminal", true)
}
