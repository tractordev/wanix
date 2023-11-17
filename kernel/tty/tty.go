package tty

import (
	"io"

	"tractor.dev/wanix/internal/jsutil"
	"tractor.dev/wanix/kernel/proc"
)

type Service struct {
	Proc *proc.Service
}

// todo: env, dir, rows, cols, term
func (s *Service) Open(path string, args []string) (*proc.Process, io.ReadWriteCloser, error) {
	// rows, cols, term would just be added to env
	p, err := s.Proc.Spawn(path, args, nil, "")
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
