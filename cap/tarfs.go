package cap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"path"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/tarfs"
	"tractor.dev/wanix/internal"
)

func tarfsAllocator() Allocator {
	return func(r *Resource) (Mounter, error) {
		return func(args []string) (fs.FS, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("tarfs: expected 1 argument, got %d", len(args))
			}
			u, err := internal.ParseURL(args[0])
			if err != nil {
				return nil, fmt.Errorf("tarfs: invalid URL: %w", err)
			}
			switch u.Scheme {
			case "http", "https":
				resp, err := http.Get(u.String())
				if err != nil {
					return nil, fmt.Errorf("tarfs: failed to download %s: %w", u.String(), err)
				}
				var reader io.ReadCloser
				if isGzipped(resp) {
					reader, err = gzip.NewReader(resp.Body)
					if err != nil {
						return nil, fmt.Errorf("tarfs: failed to create gzip reader: %w", err)
					}
				} else {
					reader = resp.Body
				}
				defer resp.Body.Close()
				return tarfs.Load(tar.NewReader(reader)), nil
			case "file":
				return nil, fmt.Errorf("tarfs: TODO: %s scheme", u.Scheme)
			default:
				return nil, fmt.Errorf("tarfs: unsupported scheme: %s", u.Scheme)
			}
		}, nil
	}
}

func isGzipped(resp *http.Response) bool {
	return resp.Header.Get("Content-Type") == "application/x-gzip" ||
		path.Ext(resp.Request.URL.Path) == ".gz" ||
		path.Ext(resp.Request.URL.Path) == ".tgz"
}
