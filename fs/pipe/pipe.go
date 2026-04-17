package pipe

// New creates a synchronous, in-memory, full duplex network connection.
func New(block bool) (*Port, *Port) {
	p1 := NewBuffer(block)
	p2 := NewBuffer(block)

	c1 := &Port{
		reader: p1,
		writer: p2,
	}
	c2 := &Port{
		reader: p2,
		writer: p1,
	}

	return c1, c2
}

type Port struct {
	reader *Buffer
	writer *Buffer
}

func (p *Port) Read(b []byte) (n int, err error) {
	return p.reader.Read(b)
}

func (p *Port) Write(b []byte) (n int, err error) {
	return p.writer.Write(b)
}

func (p *Port) Close() error {
	err1 := p.reader.Close()
	err2 := p.writer.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (p *Port) Size() int {
	return p.reader.Size()
}
