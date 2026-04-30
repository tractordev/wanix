//go:build !js

package misc

import "net/url"

func ParseURL(u string) (*url.URL, error) {
	return url.Parse(u)
}
