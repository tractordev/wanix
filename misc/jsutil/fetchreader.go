//go:build js && wasm

package jsutil

import (
	"errors"
	"io"
	"syscall/js"
)

// FetchReader implements io.ReadCloser for fetch response bodies
type FetchReader struct {
	reader   js.Value
	done     bool
	buffer   []byte
	position int
}

// Read implements io.Reader
func (fr *FetchReader) Read(p []byte) (n int, err error) {
	if fr.done {
		return 0, io.EOF
	}

	// If we have leftover data in buffer, use it first
	if fr.position < len(fr.buffer) {
		n = copy(p, fr.buffer[fr.position:])
		fr.position += n
		if fr.position >= len(fr.buffer) {
			// Buffer exhausted, clear it
			fr.buffer = nil
			fr.position = 0
		}
		return n, nil
	}

	// Need to read more data from the stream
	readPromise := fr.reader.Call("read")

	// Convert Promise to a channel using a callback
	resultChan := make(chan js.Value, 1)
	errorChan := make(chan js.Value, 1)

	readPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		resultChan <- args[0]
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		errorChan <- args[0]
		return nil
	}))

	// Wait for the promise to resolve
	select {
	case result := <-resultChan:
		done := result.Get("done").Bool()
		if done {
			fr.done = true
			return 0, io.EOF
		}

		// Get the value (Uint8Array)
		value := result.Get("value")
		length := value.Get("length").Int()

		// Copy data from Uint8Array to Go slice
		data := make([]byte, length)
		js.CopyBytesToGo(data, value)

		// Copy what we can to the caller's buffer
		n = copy(p, data)

		// If there's leftover data, store it in our buffer
		if n < length {
			fr.buffer = data[n:]
			fr.position = 0
		}

		return n, nil

	case jsErr := <-errorChan:
		return 0, errors.New("fetch read error: " + jsErr.String())
	}
}

// Close implements io.Closer
func (fr *FetchReader) Close() error {
	if !fr.done && !fr.reader.IsNull() {
		// Cancel the reader if it supports it
		if !fr.reader.Get("cancel").IsUndefined() {
			fr.reader.Call("cancel")
		}
	}
	fr.done = true
	return nil
}

// FetchToReader performs a fetch and returns an io.ReadCloser
func FetchToReader(url string) (io.ReadCloser, error) {
	// Get the global fetch function
	global := js.Global()
	fetch := global.Get("fetch")

	if fetch.IsUndefined() {
		return nil, errors.New("fetch is not available")
	}

	// Perform the fetch
	fetchPromise := fetch.Invoke(url)

	// Convert Promise to a channel
	responseChan := make(chan js.Value, 1)
	errorChan := make(chan js.Value, 1)

	fetchPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		responseChan <- args[0]
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		errorChan <- args[0]
		return nil
	}))

	// Wait for the fetch to complete
	select {
	case response := <-responseChan:
		// Check if response is ok
		if !response.Get("ok").Bool() {
			status := response.Get("status").Int()
			statusText := response.Get("statusText").String()
			return nil, errors.New("HTTP " + string(rune(status)) + ": " + statusText)
		}

		// Get the response body as a ReadableStream
		body := response.Get("body")
		if body.IsNull() {
			return nil, errors.New("response body is null")
		}

		// Get the reader from the stream
		reader := body.Call("getReader")

		return &FetchReader{
			reader: reader,
			done:   false,
		}, nil

	case jsErr := <-errorChan:
		return nil, errors.New("fetch error: " + jsErr.String())
	}
}
