//go:build !js

package internal

import "net/url"

func ParseURL(u string) (*url.URL, error) {
	return url.Parse(u)
}
