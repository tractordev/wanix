package term

import (
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/fs/pipe"
	"tractor.dev/wanix/fs/signal"
)

// Resource is one terminal instance (paths data, program, winch).
// MapFS is embedded so Route reaches the signal FS (for fs.OpenFile with O_WRONLY, etc.).
type Resource struct {
	id string
	fskit.MapFS
	hub *signal.Broadcaster
	end *pipe.Port
}

func (r *Resource) ID() string {
	return r.id
}

func (r *Resource) shutdown() {
	r.hub.Close()
	if r.end != nil {
		r.end.Close()
	}
}

type programFile struct {
	*pipe.PortFile
	prev byte // last input byte seen, for cross-call lookbehind
}

func (c *programFile) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := make([]byte, 0, len(p)+len(p)/16)
	prev := c.prev
	for _, b := range p {
		if b == '\n' && prev != '\r' {
			buf = append(buf, '\r')
		}
		buf = append(buf, b)
		prev = b
	}
	if _, err := c.PortFile.Write(buf); err != nil {
		return 0, err
	}
	c.prev = prev
	return len(p), nil
}
