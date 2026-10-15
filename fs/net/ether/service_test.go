package ether

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/progrium/go-netstack/vnet"
	"tractor.dev/wanix/fs"
)

const (
	testGatewayIP  = "192.168.127.1"
	testGatewayMAC = "5a:94:ef:e4:0c:dd"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	cfg := &vnet.Configuration{
		MTU:               1500,
		Subnet:            "192.168.127.0/24",
		GatewayIP:         testGatewayIP,
		GatewayMacAddress: testGatewayMAC,
		GatewayVirtualIPs: []string{},
	}
	vn, err := vnet.New(cfg)
	if err != nil {
		t.Fatalf("vnet: %v", err)
	}
	s := New(vn, cfg)
	t.Cleanup(func() { s.Close() })
	return s
}

func openClone(t *testing.T, s *Service) (fs.File, string) {
	t.Helper()
	f, err := s.Open("clone")
	if err != nil {
		t.Fatalf("open clone: %v", err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read clone: %v", err)
	}
	return f, strings.TrimSpace(string(b))
}

func arpRequest(srcMAC net.HardwareAddr, srcIP, dstIP net.IP) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	b.Write(srcMAC)
	b.Write([]byte{0x08, 0x06})             // ethertype ARP
	b.Write([]byte{0x00, 0x01, 0x08, 0x00}) // htype ethernet, ptype ipv4
	b.Write([]byte{6, 4, 0x00, 0x01})       // hlen, plen, op request
	b.Write(srcMAC)
	b.Write(srcIP.To4())
	b.Write(make([]byte, 6))
	b.Write(dstIP.To4())
	return b.Bytes()
}

func TestCloneReturnsCtl(t *testing.T) {
	s := newTestService(t)
	f1, id1 := openClone(t, s)
	defer f1.Close()
	f2, id2 := openClone(t, s)
	defer f2.Close()
	if id1 == "" || id1 == id2 {
		t.Fatalf("expected distinct ids, got %q %q", id1, id2)
	}
	fi, err := f1.Stat()
	if err != nil || fi.Name() != "ctl" {
		t.Fatalf("expected ctl stat, got %v %v", fi, err)
	}
	for _, name := range []string{"ctl", "data", "type", "stats"} {
		if _, err := fs.Stat(s, id1+"/"+name); err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
	}
	if _, err := fs.Write(f1, []byte("connect -1\n")); err != nil {
		t.Fatalf("connect -1: %v", err)
	}
	if _, err := fs.Write(f1, []byte("connect 2048\n")); err == nil {
		t.Fatalf("expected connect 2048 to fail")
	}
	if _, err := fs.Write(f1, []byte("bogus\n")); err == nil {
		t.Fatalf("expected unknown command to fail")
	}
	b, err := fs.ReadFile(s, id1+"/type")
	if err != nil || string(b) != "-1\n" {
		t.Fatalf("type: %q %v", b, err)
	}
	b, err = fs.ReadFile(s, "addr")
	if err != nil || string(b) != "5a94efe40cdd\n" {
		t.Fatalf("addr: %q %v", b, err)
	}
}

func TestLifetimeTiedToOpenFiles(t *testing.T) {
	s := newTestService(t)
	ctl, id := openClone(t, s)
	data, err := s.Open(id + "/data")
	if err != nil {
		t.Fatalf("open data: %v", err)
	}
	c, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	ctl.Close()
	if _, err := fs.Stat(s, id); err != nil {
		t.Fatalf("conversation freed while data open: %v", err)
	}
	data.Close()
	if _, err := fs.Stat(s, id); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected conversation removed, got %v", err)
	}
	select {
	case <-c.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("AcceptQemu context not canceled")
	}
}

func TestARPGateway(t *testing.T) {
	s := newTestService(t)
	ctl, id := openClone(t, s)
	defer ctl.Close()
	data, err := s.Open(id + "/data")
	if err != nil {
		t.Fatalf("open data: %v", err)
	}
	defer data.Close()

	mac, _ := net.ParseMAC("52:54:00:12:34:56")
	req := arpRequest(mac, net.ParseIP("192.168.127.2"), net.ParseIP(testGatewayIP))
	if _, err := fs.Write(data, req); err != nil {
		t.Fatalf("write arp: %v", err)
	}

	gw, _ := net.ParseMAC(testGatewayMAC)
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := data.Read(buf)
			if err != nil {
				close(got)
				return
			}
			f := buf[:n]
			if n >= 42 && bytes.Equal(f[12:14], []byte{0x08, 0x06}) && f[21] == 2 {
				got <- append([]byte(nil), f...)
				return
			}
		}
	}()
	select {
	case f, ok := <-got:
		if !ok {
			t.Fatal("data closed before arp reply")
		}
		if !bytes.Equal(f[22:28], gw) {
			t.Fatalf("arp reply from %x, want %x", f[22:28], gw)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for arp reply")
	}
}

func TestHangup(t *testing.T) {
	s := newTestService(t)
	ctl, id := openClone(t, s)
	defer ctl.Close()
	data, err := s.Open(id + "/data")
	if err != nil {
		t.Fatalf("open data: %v", err)
	}
	defer data.Close()

	readErr := make(chan error, 1)
	go func() {
		_, err := data.Read(make([]byte, 2048))
		readErr <- err
	}()
	if _, err := fs.Write(ctl, []byte("hangup\n")); err != nil {
		t.Fatalf("hangup: %v", err)
	}
	select {
	case err := <-readErr:
		if err != io.EOF {
			t.Fatalf("expected EOF, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pending read not unblocked by hangup")
	}
	if _, err := fs.Write(data, make([]byte, 60)); err == nil {
		t.Fatal("expected write after hangup to fail")
	}
	if _, err := fs.Stat(s, id); err != nil {
		t.Fatalf("conversation removed while handles open: %v", err)
	}
	b, err := fs.ReadFile(s, id+"/stats")
	if err != nil || !strings.Contains(string(b), "state: closed") {
		t.Fatalf("stats: %q %v", b, err)
	}
}

func TestFrameSizeLimits(t *testing.T) {
	s := newTestService(t)
	ctl, id := openClone(t, s)
	defer ctl.Close()
	data, err := s.Open(id + "/data")
	if err != nil {
		t.Fatalf("open data: %v", err)
	}
	defer data.Close()
	if _, err := fs.Write(data, make([]byte, 13)); err == nil {
		t.Fatal("expected undersized frame to fail")
	}
	if _, err := fs.Write(data, make([]byte, 1500+15)); err == nil {
		t.Fatal("expected oversized frame to fail")
	}
}
