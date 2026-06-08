package test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExamplesHTML(t *testing.T) {
	os.Chdir("..")
	// Serve project over HTTP on port 7072
	fs := http.FileServer(http.Dir("."))
	l, err := net.Listen("tcp4", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	host, portStr, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("Failed to get listen port: %v", err)
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}
	baseURL := fmt.Sprintf("http://%s:%s", host, portStr)
	srv := &http.Server{
		Handler: fs,
	}
	ready := make(chan struct{})
	go func() {
		close(ready)
		fmt.Printf("Serving on %s\n", baseURL)
		if err := srv.Serve(l); err != http.ErrServerClosed {
			t.Fatalf("Failed to start server: %v", err)
		}
		fmt.Println("Server stopped")
	}()
	<-ready

	// Wait for the server to actually be available
	ok := false
	for i := 0; i < 20; i++ {
		resp, err := http.Get(baseURL)
		if err == nil && resp.StatusCode == 200 {
			ok = true
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ok {
		srv.Shutdown(context.Background())
		t.Fatalf("Failed to confirm server is up")
	}

	// For cleanup
	defer srv.Shutdown(context.Background())

	htmlFiles := []string{}
	err = filepath.Walk("./examples", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			htmlFiles = append(htmlFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to list example html files: %v", err)
	}

	for _, filename := range htmlFiles {
		filename := filename // capture
		t.Run(filename, func(t *testing.T) {
			// t.Parallel()
			url := fmt.Sprintf("%s/%s", baseURL, strings.ReplaceAll(filename, `\`, `/`))
			fmt.Println(url)
			cmd := exec.Command("go", "run", "main.go", url)
			cmd.Dir = "./test/wtest"
			cmd.Env = append(os.Environ(), "GOWORK=off")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Error running test for %s: %v\nOutput:\n%s", filename, err, string(out))
			}
		})
	}
}
