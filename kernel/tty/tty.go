package tty

import (
	"io"
	"strconv"

	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/kernel/proc"
)

type Service struct {
	Proc *proc.Service

	defaultCols int
	defaultRows int
}

func (s *Service) Initialize() {
	s.defaultCols = 80
	s.defaultRows = 24
}

func (s *Service) Open(path string, args []string, env map[string]string) (*proc.Process, io.ReadWriteCloser, error) {
	if env == nil {
		env = map[string]string{}
	}
	if env["TERM"] == "" {
		env["TERM"] = "xterm-256color"
	}
	if env["COLS"] == "" {
		env["COLS"] = strconv.Itoa(s.defaultCols)
	}
	if env["ROWS"] == "" {
		env["ROWS"] = strconv.Itoa(s.defaultRows)
	}
	p, err := s.Proc.Spawn(path, args, env, "")
	if err != nil {
		return nil, nil, err
	}
	stdin := p.Stdin()
	output := p.Output()
	return p, &jsutil.ReadWriter{
		ReadCloser:  output,
		WriteCloser: stdin,
	}, nil
}

func (s *Service) Resize(pid, rows, cols int) {
	// TODO: winch signal, per tty sizes
	s.defaultCols = cols
	s.defaultRows = rows
}
