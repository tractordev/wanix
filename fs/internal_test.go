package fs

import (
	"errors"
	"testing"
)

func Test_OpErr(t *testing.T) {
	e := opErr(nil, "test", "test", ErrNotSupported)
	if !errors.Is(e, ErrNotSupported) {
		t.Errorf("expected ErrNotSupported, got %v", e)
	}
}
