package fs

import "strings"

// BindOption is a bind modifier. Tags without "=" are keys with an empty value;
// "key=value" sets both key and value in the parsed options map.
type BindOption string

const (
	BindReplace BindOption = "replace"
	BindAfter   BindOption = "after"
	BindBefore  BindOption = "before"
)

// BindType is a bind type value (e.g. "ns") or type option (e.g. "type=ns").
type BindType = BindOption

const (
	// BindNS sets type=ns. Omitted by default — namespace binds assume ns.
	BindNS BindType = "type=ns"
)

// ParseBindOptions parses bind options into a map. Options without "=" are
// tags (key with empty value). Later options overwrite earlier keys.
func ParseBindOptions(opts ...BindOption) map[string]string {
	m := make(map[string]string, len(opts))
	for _, o := range opts {
		s := string(o)
		if k, v, ok := strings.Cut(s, "="); ok {
			m[k] = v
		} else {
			m[s] = ""
		}
	}
	return m
}

// BindPlacement returns the first after/before/replace option in opts, or BindAfter.
func BindPlacement(opts ...BindOption) BindOption {
	for _, o := range opts {
		switch o {
		case BindAfter, BindBefore, BindReplace:
			return o
		}
	}
	return BindAfter
}

// BindTypeOf returns the bind type value from opts (the type= value). Default is "ns".
func BindTypeOf(opts ...BindOption) BindType {
	if v := ParseBindOptions(opts...)["type"]; v != "" {
		return BindType(v)
	}
	return "ns"
}

// BindFS is implemented by filesystems that support Plan9-style bind operations.
type BindFS interface {
	FS
	Bind(src FS, srcPath, dstPath string, opts ...BindOption) error
}

// UnbindFS is implemented by filesystems that support removing bindings.
type UnbindFS interface {
	FS
	Unbind(src FS, srcPath, dstPath string) error
}

// Bind adds a file or directory binding if supported.
func Bind(fsys FS, src FS, srcPath, dstPath string, opts ...BindOption) error {
	if b, ok := fsys.(BindFS); ok {
		return b.Bind(src, srcPath, dstPath, opts...)
	}
	return opErr(fsys, dstPath, "bind", ErrNotSupported)
}

// Unbind removes a binding if supported.
func Unbind(fsys FS, src FS, srcPath, dstPath string) error {
	if u, ok := fsys.(UnbindFS); ok {
		return u.Unbind(src, srcPath, dstPath)
	}
	return opErr(fsys, dstPath, "unbind", ErrNotSupported)
}
