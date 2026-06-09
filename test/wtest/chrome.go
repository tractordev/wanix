package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const chromeStartTimeout = 15 * time.Second

func chromeDebugURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func chromeDebugReady(ctx context.Context, port int) bool {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:%d/json/version", port),
		nil,
	)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil || v.WebSocketDebuggerURL == "" {
		return false
	}
	return true
}

func chromeExecPath() (string, error) {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default:
		paths = []string{
			"google-chrome-stable",
			"google-chrome",
			"chromium-browser",
			"chromium",
			"chrome",
		}
	}
	for _, path := range paths {
		if full, err := exec.LookPath(path); err == nil {
			return full, nil
		}
	}
	return "", fmt.Errorf("chrome executable not found")
}

func chromeUserDataDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "wtest", "chrome-profile"), nil
}

func startChrome(port int) error {
	execPath, err := chromeExecPath()
	if err != nil {
		return err
	}
	userDataDir, err := chromeUserDataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return err
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=127.0.0.1",
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"about:blank",
	}
	cmd := exec.Command(execPath, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

// ensureChrome returns a DevTools HTTP endpoint for an existing Chrome on port,
// or starts a detached Chrome and waits until it is ready.
func ensureChrome(ctx context.Context, port int) (endpoint string, started bool, err error) {
	endpoint = chromeDebugURL(port)
	if chromeDebugReady(ctx, port) {
		return endpoint, false, nil
	}

	if err := startChrome(port); err != nil {
		return "", false, fmt.Errorf("start chrome: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, chromeStartTimeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if chromeDebugReady(waitCtx, port) {
			return endpoint, true, nil
		}
		select {
		case <-waitCtx.Done():
			return "", true, fmt.Errorf("chrome on port %d not ready: %w", port, waitCtx.Err())
		case <-ticker.C:
		}
	}
}
