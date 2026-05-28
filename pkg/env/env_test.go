package env

import (
	"context"
	"testing"

	"github.com/samson-samson/aievo-next/pkg/event"
)

func TestInMemory_AppendAndLoad_AllFilters(t *testing.T) {
	m := NewInMemory()
	ctx := context.Background()
	msgs := []event.MessageEvent{
		event.NewMessageEvent("alice", []string{"bob"}, "hi"),
		event.NewMessageEvent("bob", []string{"alice"}, "yo"),
		event.NewMessageEvent("alice", nil, "all hands"),
		event.NewMessageEvent("carol", []string{"bob"}, "ping"),
	}
	for _, msg := range msgs {
		if _, err := m.Append(ctx, msg); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.Load(ctx, Filter{Receiver: "bob", After: -1})
	if err != nil {
		t.Fatal(err)
	}
	// bob should see: alice→bob, alice→all (broadcast), carol→bob  = 3
	if len(got) != 3 {
		t.Fatalf("bob loaded %d, want 3 (alice→bob, broadcast, carol→bob)", len(got))
	}
}

func TestInMemory_LoadAfter(t *testing.T) {
	m := NewInMemory()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = m.Append(ctx, event.NewMessageEvent("a", nil, "x"))
	}
	got, _ := m.Load(ctx, Filter{After: 2})
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (indexes 3 and 4)", len(got))
	}
}

func TestInMemory_RespectsContextCancel(t *testing.T) {
	m := NewInMemory()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Append(ctx, event.NewMessageEvent("a", nil, "x")); err == nil {
		t.Fatal("expected ctx.Canceled")
	}
}
