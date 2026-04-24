package signal

import "sync"

// NoExclude is the exclude id for Broadcast when every subscriber should receive the message.
const NoExclude int64 = -1

// Broadcaster fans out byte frames to multiple subscribers. Each subscriber
// has a buffered channel; writers block on full buffers. It is a general
// primitive for pub/sub style signals (for example terminal SIGWINCH payloads).
type Broadcaster struct {
	mu      sync.Mutex
	readers map[int64]chan []byte
	nextID  int64
}

// NewBroadcaster returns an empty broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{readers: make(map[int64]chan []byte)}
}

// AddReader registers a subscriber and returns an id and delivery channel.
func (b *Broadcaster) AddReader() (id int64, ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id = b.nextID
	ch = make(chan []byte, 64)
	b.readers[id] = ch
	return id, ch
}

// RemoveReader unsubscribes and closes the subscriber channel.
func (b *Broadcaster) RemoveReader(id int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.readers[id]; ok {
		delete(b.readers, id)
		close(ch)
	}
}

// Broadcast delivers a copy of data to every subscriber except exclude when exclude >= 0.
func (b *Broadcaster) Broadcast(data []byte, exclude int64) {
	b.mu.Lock()
	var targets []chan []byte
	for id, ch := range b.readers {
		if exclude >= 0 && id == exclude {
			continue
		}
		targets = append(targets, ch)
	}
	b.mu.Unlock()
	payload := append([]byte(nil), data...)
	for _, ch := range targets {
		ch <- payload
	}
}

// Close shuts down all subscribers and clears state.
func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.readers {
		close(ch)
	}
	b.readers = make(map[int64]chan []byte)
}

// SubscriberCount returns the number of active reader subscriptions.
func (b *Broadcaster) SubscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.readers)
}
