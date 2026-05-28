package event

import (
	"context"
	"sync"
)

// Bus is a typed, fan-out event bus.
//
// Why not just channels? Because we want N subscribers per topic without
// each producer caring about subscribers, and we want graceful close.
type Bus struct {
	mu     sync.RWMutex
	subs   []chan Event
	closed bool
}

func NewBus() *Bus { return &Bus{} }

// Subscribe returns a read-only channel that receives every event published
// after Subscribe returns. The buffer size protects slow consumers from
// blocking publishers — but a consumer that lags by more than `buffer`
// events will drop the oldest (we choose drop-oldest to preserve liveness).
func (b *Bus) Subscribe(buffer int) <-chan Event {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan Event, buffer)
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(ch)
		return ch
	}
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch
}

// Publish fans out `e` to all subscribers. ctx lets the caller bail out
// if the bus is being shut down concurrently. Slow subscribers are
// non-blocking: we drop the oldest enqueued event.
func (b *Bus) Publish(ctx context.Context, e Event) {
	b.mu.RLock()
	subs := b.subs
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return
	}
	for _, ch := range subs {
		select {
		case <-ctx.Done():
			return
		case ch <- e:
		default:
			// Buffer full → drop oldest, then enqueue. This preserves liveness
			// at the cost of historic events for slow consumers (acceptable
			// because the bus is not the persistence layer — Memory is).
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- e:
			default:
			}
		}
	}
}

// Close stops accepting new events and closes every subscriber channel.
// Safe to call multiple times.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, ch := range b.subs {
		close(ch)
	}
	b.subs = nil
}
