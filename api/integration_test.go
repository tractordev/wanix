package api

import (
	"context"
	"testing"

	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/wanix"
	"tractor.dev/wanix/fs/memfs"
)

// startTestServer wires a Responder on one end of an in-memory duplex session,
// with a root task whose namespace is a fresh memfs, and returns a client peer.
func startTestServer(t *testing.T) *talk.Peer {
	t.Helper()
	root, err := wanix.NewRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := root.NS().Bind(memfs.New(), ".", "."); err != nil {
		t.Fatal(err)
	}
	a, b := mux.Pair()
	t.Cleanup(func() { a.Close(); b.Close() })
	go Responder(a, root)
	return talk.NewPeer(b, codec.CBORCodec{})
}

func TestGuardRecoversPanic(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	// Chmod panics on a bad mode type; the guard must recover it and return
	// an error instead of crashing the kernel.
	if _, err := client.Call(ctx, "Chmod", []any{"hello.txt", "bad"}); err == nil {
		t.Fatal("expected error from guarded panic")
	}

	// The kernel must still serve subsequent calls.
	if _, err := client.Call(ctx, "WriteFile", []any{"after.txt", []byte("ok")}); err != nil {
		t.Fatalf("server died after a guarded panic: %v", err)
	}
}

func TestChtimesIntegerTimestamps(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	if _, err := client.Call(ctx, "WriteFile", []any{"hello.txt", []byte("hi")}); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	// Integer timestamps: CBOR encodes whole-number seconds as integers, which
	// is what used to panic the kernel (arg was asserted as float64).
	if _, err := client.Call(ctx, "Chtimes", []any{"hello.txt", int64(1700000000), uint64(1700000001)}); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}
