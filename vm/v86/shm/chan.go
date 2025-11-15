package shm

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	SharedMemAddr = 0x3F000000
	SharedMemSize = 0x1000000 // 16MB

	ControlSize = 0x1000
	BufferSize  = (SharedMemSize - ControlSize) / 2

	// Control structure offsets
	JsGoHead    = 0
	JsGoTail    = 4
	GoJsHead    = 8
	GoJsTail    = 12
	StatusFlags = 16

	// Buffer offsets
	JsGoBuffer = ControlSize
	GoJsBuffer = ControlSize + BufferSize
)

type SharedChannel struct {
	mem []byte

	// Pointers to control values
	jsGoHead *uint32
	jsGoTail *uint32
	goJsHead *uint32
	goJsTail *uint32

	// Buffer slices
	jsGoBuffer []byte
	goJsBuffer []byte

	// Partial read state (for stream-like semantics)
	readBuffer []byte // holds current message being read
	readPos    int    // current position within readBuffer
}

func NewSharedChannel() (*SharedChannel, error) {
	file, err := os.OpenFile("/dev/mem", os.O_RDWR|os.O_SYNC, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/mem: %w", err)
	}
	defer file.Close()

	mem, err := syscall.Mmap(
		int(file.Fd()),
		SharedMemAddr,
		SharedMemSize,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		return nil, fmt.Errorf("mmap failed: %w", err)
	}

	ch := &SharedChannel{
		mem: mem,

		jsGoHead: (*uint32)(unsafe.Pointer(&mem[JsGoHead])),
		jsGoTail: (*uint32)(unsafe.Pointer(&mem[JsGoTail])),
		goJsHead: (*uint32)(unsafe.Pointer(&mem[GoJsHead])),
		goJsTail: (*uint32)(unsafe.Pointer(&mem[GoJsTail])),

		jsGoBuffer: mem[JsGoBuffer : JsGoBuffer+BufferSize],
		goJsBuffer: mem[GoJsBuffer : GoJsBuffer+BufferSize],
	}

	// fmt.Printf("SharedChannel initialized at 0x%x\n", SharedMemAddr)
	return ch, nil
}

func (ch *SharedChannel) Close() error {
	return syscall.Munmap(ch.mem)
}

// Read implements io.Reader - reads from JS->Go buffer
// Uses head/tail pointers for synchronization
// Implements stream-like semantics: if a message is larger than the provided buffer,
// it will be read in multiple calls until fully consumed.
func (ch *SharedChannel) Read(p []byte) (n int, err error) {
	// First check if we have data from a previous partial read
	if ch.readBuffer != nil && ch.readPos < len(ch.readBuffer) {
		// Copy what fits from the remaining data
		n = copy(p, ch.readBuffer[ch.readPos:])
		ch.readPos += n

		// If we've consumed the entire message, clear the buffer
		if ch.readPos >= len(ch.readBuffer) {
			ch.readBuffer = nil
			ch.readPos = 0
		}

		return n, nil
	}

	// No partial data, read a new message from shared memory
	waitCount := 0
	for {
		// Read head and tail pointers (use atomic reads to ensure visibility)
		head := atomic.LoadUint32(ch.jsGoHead)
		tail := atomic.LoadUint32(ch.jsGoTail)

		// Use unsigned subtraction to handle uint32 overflow correctly
		dataAvailable := head - tail // Works even if head < tail due to overflow

		// Check if there's a message
		if dataAvailable > 0 {
			if waitCount > 0 {
				// fmt.Printf("[SharedChannel.Read] Message available after %d waits (head=%d, tail=%d)\n",
				// 	waitCount, head, tail)
			}
			waitCount = 0

			// Calculate buffer positions using modulo
			tailPos := tail % BufferSize

			// Read message length at tail position (using modulo)
			if tailPos+4 > BufferSize {
				// Not enough space for length header, skip to next cycle
				newTail := ((tail / BufferSize) + 1) * BufferSize
				atomic.StoreUint32(ch.jsGoTail, newTail)
				continue
			}

			lengthBytes := ch.jsGoBuffer[tailPos : tailPos+4]
			length := binary.LittleEndian.Uint32(lengthBytes)

			// Check if we have a valid message
			if length > 0 && length < BufferSize {
				// Check if full message fits in buffer
				if tailPos+4+length <= BufferSize {
					// Read the ENTIRE message into internal buffer
					ch.readBuffer = make([]byte, length)
					copy(ch.readBuffer, ch.jsGoBuffer[tailPos+4:tailPos+4+length])
					ch.readPos = 0

					// Update tail pointer (using logical position, not modulo)
					newTail := tail + 4 + length
					atomic.StoreUint32(ch.jsGoTail, newTail)

					// fmt.Println("inner: read message:", length, "bytes, head:", head, "tail:", tail)

					// Now copy what fits into caller's buffer
					n = copy(p, ch.readBuffer)
					ch.readPos = n

					// If we've consumed the entire message, clear the buffer
					if ch.readPos >= len(ch.readBuffer) {
						ch.readBuffer = nil
						ch.readPos = 0
					}

					return n, nil
				} else {
					// Message doesn't fit at current position, must be a gap
					newTail := ((tail / BufferSize) + 1) * BufferSize
					fmt.Printf("Message doesn't fit (tailPos=%d, length=%d), skipping to %d\n", tailPos, length, newTail)
					atomic.StoreUint32(ch.jsGoTail, newTail)
				}
			} else {
				// Invalid length, skip to next cycle
				newTail := ((tail / BufferSize) + 1) * BufferSize
				// fmt.Printf("Invalid message length: %d at tailPos=%d, skipping to %d\n", length, tailPos, newTail)
				atomic.StoreUint32(ch.jsGoTail, newTail)
			}
		}

		// No message found, yield CPU and retry immediately
		waitCount++
		if waitCount%1000 == 0 {
			// fmt.Printf("[SharedChannel.Read] Waiting for message... (head=%d, tail=%d, waited %d times)\n",
			// 	atomic.LoadUint32(ch.jsGoHead), atomic.LoadUint32(ch.jsGoTail), waitCount)
		}
		time.Sleep(1 * time.Millisecond)
	}
}

// Write implements io.Writer - writes to Go->JS buffer
// Uses head/tail pointers for synchronization
// Returns error if buffer is full after timeout
func (ch *SharedChannel) Write(p []byte) (n int, err error) {
	messageSize := 4 + uint32(len(p))

	// Wait for space in buffer with timeout (max 1 second)
	deadline := time.Now().Add(1 * time.Second)
	attempts := 0

	for time.Now().Before(deadline) {
		head := atomic.LoadUint32(ch.goJsHead)
		tail := atomic.LoadUint32(ch.goJsTail)

		// Calculate available space (reserve some margin to prevent full buffer ambiguity)
		const safetyMargin = 16384 // Reserve 16KB (1% of 16MB buffer)
		var available uint32
		if head >= tail {
			// Space from head to end, plus from start to tail
			available = (BufferSize - head) + tail
		} else {
			// Space between tail and head (wrapped case)
			available = tail - head
		}

		// Subtract safety margin
		if available > safetyMargin {
			available -= safetyMargin
		} else {
			available = 0
		}

		// Check if message fits
		if messageSize <= available {
			// Calculate buffer position using modulo
			headPos := head % BufferSize

			// Check if message fits at current position
			if headPos+messageSize > BufferSize {
				// Need to skip gap - advance head to next buffer cycle
				newHead := ((head / BufferSize) + 1) * BufferSize
				tailPos := tail % BufferSize

				// Check if tail is far enough from start of buffer (using buffer positions only)
				// This avoids incorrect comparisons when uint32 overflows
				if tailPos >= messageSize {
					// Safe to skip gap and write at beginning
					head = newHead
					headPos = 0
				} else {
					// Not enough space at beginning, wait
					attempts++
					if attempts < 100 {
						runtime.Gosched()
					} else {
						time.Sleep(1 * time.Millisecond)
						attempts = 0
					}
					continue
				}
			}

			// We have space, write the message
			message := make([]byte, messageSize)
			binary.LittleEndian.PutUint32(message[:4], uint32(len(p)))
			copy(message[4:], p)

			// Write message at head position (using modulo)
			copy(ch.goJsBuffer[headPos:], message)

			// Memory barrier to ensure visibility
			runtime.Gosched()

			// Update head pointer to signal new message (logical position, not modulo)
			newHead := head + messageSize
			atomic.StoreUint32(ch.goJsHead, newHead)

			// fmt.Println("inner: write:", len(p), "head:", head, "tail:", tail)

			return len(p), nil
		}

		// Buffer full, yield and retry
		attempts++
		if attempts < 100 {
			runtime.Gosched()
		} else {
			time.Sleep(1 * time.Millisecond)
			attempts = 0
		}
	}

	// Timeout - buffer still full
	return 0, fmt.Errorf("write timeout: buffer full after 1s")
}
