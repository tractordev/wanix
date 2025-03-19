//go:build js

package internal

import (
	"net/url"
	"strings"
	"syscall/js"
)

// ParseURL wraps url.Parse adding a default scheme and host based on the current DOM location.
// todo: handle relative paths
func ParseURL(u string) (*url.URL, error) {
	uu, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if uu.Scheme == "" {
		uu.Scheme = strings.Trim(js.Global().Get("location").Get("protocol").String(), ":")
	}
	if uu.Host == "" {
		uu.Host = js.Global().Get("location").Get("host").String()
	}
	return uu, nil
}
