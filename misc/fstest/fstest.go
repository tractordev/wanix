package fstest

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"

	"github.com/hugelgupf/p9/p9"
	"github.com/u-root/uio/ulog"
	"tractor.dev/wanix/fs/p9kit"
)

func RunFor(ctx context.Context, fsys fs.FS, tags string, p9debug bool) error {
	ip, err := getIP()
	if err != nil {
		return fmt.Errorf("getIP: %v", err)
	}
	l, err := net.Listen("tcp4", ip+":0")
	if err != nil {
		return fmt.Errorf("err binding: %v", err)
	}
	addr := l.Addr().(*net.TCPAddr).String()

	go func() {
		if err := ServeFor(ctx, l, fsys, p9debug); err != nil {
			log.Fatal(err)
		}
	}()

	cmd := "testfs"
	if tags != "" {
		cmd += " " + tags
	}
	return RunTester(ctx, addr, cmd)
}

func ServeFor(ctx context.Context, l net.Listener, fsys fs.FS, p9debug bool) error {
	defer l.Close()
	var opts []p9.ServerOpt
	if p9debug {
		opts = append(opts, p9.WithServerLogger(ulog.Log))
	}
	srv := p9.NewServer(p9kit.Attacher(fsys), opts...)
	return srv.Serve(l)
}

func RunTester(ctx context.Context, addr string, cmd string) error {
	if !dockerAvailable() {
		return fmt.Errorf("docker not available")
	}
	if !dockerRunning() {
		return fmt.Errorf("docker not running")
	}
	if !imageAvailable() {
		return fmt.Errorf("fstest image not available")
	}
	return runDockerFSTest(ctx, addr, cmd)
}

// dockerAvailable returns true if the "docker" command is available in the PATH.
func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// dockerRunning returns true if the Docker daemon is running and accessible by attempting to run "docker ps".
func dockerRunning() bool {
	cmd := exec.Command("docker", "ps")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// imageAvailable returns true if the Docker image "fstest" is available locally.
func imageAvailable() bool {
	cmd := exec.Command("sh", "-c", "docker images | grep fstest")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// getIP returns the local IP address using ifconfig en0.
func getIP() (string, error) {
	cmd := exec.Command("sh", "-c", `ifconfig en0 | awk '/inet /{print $2}'`)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Output may have trailing newline
	ip := string(out)
	// Remove trailing newline(s) and spaces
	return string(bytes.TrimSpace([]byte(ip))), nil
}

func runDockerFSTest(ctx context.Context, addr string, cmd string) error {
	c := exec.CommandContext(ctx, "docker", "run", "--rm", "-it", "--privileged", "fstest", addr, cmd)
	// Pass through to real stdio so -it "works", if within a terminal.
	c.Stdout = nil
	c.Stderr = nil
	c.Stdin = nil
	if !isTerminal() {
		// Fallback: connect output to parent's stdout/stderr
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin
	}
	done := make(chan error, 1)
	go func() {
		done <- c.Run()
	}()
	select {
	case <-ctx.Done():
		_ = c.Process.Kill()
		<-done // Wait for command to actually exit
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func isTerminal() bool {
	cmd := exec.Command("tty")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
