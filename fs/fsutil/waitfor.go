package fsutil

import (
	"context"
	"errors"
	"io/fs"
	"time"
)

// WaitFor polls fsys to check if the given path exists or does not exist based on the 'exist' parameter.
// It waits up to the deadline in the context, returning ctx.Err() on timeout/cancel.
// If exist=true, waits until file exists. If exist=false, waits until file is gone.
// It checks every 500 milliseconds.
func WaitFor(ctx context.Context, fsys fs.FS, filepath string, exist bool) error {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		_, err := fs.Stat(fsys, filepath)
		fileExists := err == nil
		if exist == fileExists {
			return nil // condition met
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err // some other error
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
