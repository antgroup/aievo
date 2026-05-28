package event

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSealedInterface_TypeSwitchExhaustive(t *testing.T) {
	// Compile-time proof: a type switch over Event must accept all variants.
	classify := func(e Event) string {
		switch e.(type) {
		case AgentStartedEvent:
			return "started"
		case LLMChunkEvent:
			return "chunk"
		case ToolCallEvent:
			return "call"
		case ToolResultEvent:
			return "result"
		case MessageEvent:
			return "msg"
		case TerminalEvent:
			return "terminal"
		case ErrorEvent:
			return "error"
		default:
			t.Fatalf("unhandled event %T", e)
			return ""
		}
	}
	cases := []struct {
		e    Event
		want string
	}{
		{NewAgentStartedEvent("a", 1), "started"},
		{NewLLMChunkEvent("a", "hi"), "chunk"},
		{NewToolCallEvent("a", nil), "call"},
		{NewToolResultEvent("a", nil), "result"},
		{NewMessageEvent("a", nil, "x"), "msg"},
		{NewTerminalEvent("a", TerminalDone), "terminal"},
		{NewErrorEvent("a", errors.New("boom")), "error"},
	}
	for _, c := range cases {
		if got := classify(c.e); got != c.want {
			t.Errorf("classify(%T) = %s, want %s", c.e, got, c.want)
		}
	}
}

func TestBus_PublishFanOut(t *testing.T) {
	bus := NewBus()
	defer bus.Close()
	const subs = 3
	chans := make([]<-chan Event, subs)
	for i := range chans {
		chans[i] = bus.Subscribe(8)
	}
	want := NewMessageEvent("a", []string{"b"}, "hello")
	bus.Publish(context.Background(), want)
	for i, ch := range chans {
		select {
		case got := <-ch:
			if m, ok := got.(MessageEvent); !ok || m.Content != want.Content {
				t.Errorf("sub %d: got %v", i, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub %d: timeout", i)
		}
	}
}

func TestBus_SlowConsumerDoesNotBlockPublisher(t *testing.T) {
	bus := NewBus()
	defer bus.Close()
	_ = bus.Subscribe(1) // never read
	for i := 0; i < 1000; i++ {
		bus.Publish(context.Background(), NewAgentStartedEvent("a", i))
	}
	// If publish blocked, we'd hang above.
}

func TestBus_Close_CloseChannels(t *testing.T) {
	bus := NewBus()
	ch := bus.Subscribe(2)
	bus.Close()
	// Reading from a closed empty channel returns zero value with ok=false.
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed")
	}
	bus.Close() // double-close must be safe
}

func TestBus_ConcurrentPublishSubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()
	const n = 200
	var received int64
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		ch := bus.Subscribe(64)
		go func() {
			defer wg.Done()
			for range ch {
				atomic.AddInt64(&received, 1)
			}
		}()
	}
	for i := 0; i < n; i++ {
		bus.Publish(context.Background(), NewAgentStartedEvent("a", i))
	}
	bus.Close()
	wg.Wait()
	if received == 0 {
		t.Fatal("no events received")
	}
}
