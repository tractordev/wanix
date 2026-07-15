package rc

// Watch command scaffolding (issue #247): re-run a command when files change.
// Full FS watcher integration can land in a follow-up; this provides the CLI surface.

import (
        "fmt"
        "os"
        "os/exec"
        "time"
)

// RunWatch repeatedly runs args[0] with args[1:] on an interval.
// Usage: wanix rc watch [-n seconds] <command> [args...]
func RunWatch(args []string) error {
        interval := 2 * time.Second
        if len(args) >= 2 && (args[0] == "-n" || args[0] == "--interval") {
                var sec int
                _, err := fmt.Sscanf(args[1], "%d", &sec)
                if err == nil && sec > 0 {
                        interval = time.Duration(sec) * time.Second
                }
                args = args[2:]
        }
        if len(args) == 0 {
                return fmt.Errorf("watch: missing command")
        }
        for {
                cmd := exec.Command(args[0], args[1:]...)
                cmd.Stdout = os.Stdout
                cmd.Stderr = os.Stderr
                cmd.Stdin = os.Stdin
                _ = cmd.Run()
                time.Sleep(interval)
        }
}
