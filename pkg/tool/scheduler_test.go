package tool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samson-samson/aievo-next/pkg/featureflag"
)

// fakeTool sleeps for d, optionally returning err. Safe flag determines
// whether the scheduler will parallelise.
type fakeTool struct {
	name  string
	d     time.Duration
	safe  bool
	calls int64
	err   error
}

func (f *fakeTool) Name() string             { return f.name }
func (f *fakeTool) Description() string      { return f.name }
func (f *fakeTool) Schema() []byte           { return nil }
func (f *fakeTool) IsConcurrencySafe() bool  { return f.safe }
func (f *fakeTool) Call(ctx context.Context, input string) (string, error) {
	atomic.AddInt64(&f.calls, 1)
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(f.d):
	}
	if f.err != nil {
		return "", f.err
	}
	return f.name + ":" + input, nil
}

func TestExecuteCalls_SafeRunInParallel(t *testing.T) {
	t.Cleanup(featureflag.Reload)
	featureflag.Set(featureflag.FlagParallelTools, true)
	tool := &fakeTool{name: "Read", d: 80 * time.Millisecond, safe: true}
	calls := []Call{
		{ID: "1", Name: "Read", Input: "a"},
		{ID: "2", Name: "Read", Input: "b"},
		{ID: "3", Name: "Read", Input: "c"},
		{ID: "4", Name: "Read", Input: "d"},
	}
	start := time.Now()
	results, err := ExecuteCalls(context.Background(), calls, []Tool{tool}, SchedulerConfig{MaxParallel: 4})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	// 4 calls × 80ms serial would be 320ms+; parallel should be well under 200ms.
	if elapsed > 200*time.Millisecond {
		t.Fatalf("parallel safe calls took %v, expected <200ms", elapsed)
	}
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}
	// Verify *model order* preserved despite parallel execution.
	for i, r := range results {
		if r.CallID != calls[i].ID {
			t.Errorf("result %d CallID = %s, want %s", i, r.CallID, calls[i].ID)
		}
	}
}

func TestExecuteCalls_UnsafeAreSequential(t *testing.T) {
	t.Cleanup(featureflag.Reload)
	featureflag.Set(featureflag.FlagParallelTools, true)
	tool := &fakeTool{name: "Bash", d: 50 * time.Millisecond, safe: false}
	calls := []Call{
		{ID: "1", Name: "Bash", Input: "x"},
		{ID: "2", Name: "Bash", Input: "y"},
		{ID: "3", Name: "Bash", Input: "z"},
	}
	start := time.Now()
	results, err := ExecuteCalls(context.Background(), calls, []Tool{tool}, SchedulerConfig{MaxParallel: 8})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed < 140*time.Millisecond {
		t.Fatalf("unsafe calls finished in %v — looks parallel, expected sequential ≥150ms", elapsed)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results", len(results))
	}
}

func TestExecuteCalls_FeatureFlagDisablesParallel(t *testing.T) {
	t.Cleanup(featureflag.Reload)
	featureflag.Set(featureflag.FlagParallelTools, false)
	tool := &fakeTool{name: "Read", d: 60 * time.Millisecond, safe: true}
	calls := []Call{
		{ID: "1", Name: "Read", Input: "a"},
		{ID: "2", Name: "Read", Input: "b"},
	}
	start := time.Now()
	_, err := ExecuteCalls(context.Background(), calls, []Tool{tool}, SchedulerConfig{MaxParallel: 4})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("flag-off elapsed=%v, expected sequential ≥120ms", elapsed)
	}
}

func TestExecuteCalls_UnknownToolFailsFast(t *testing.T) {
	t.Cleanup(featureflag.Reload)
	tool := &fakeTool{name: "Read", d: time.Millisecond, safe: true}
	calls := []Call{
		{ID: "1", Name: "Nonexistent", Input: ""},
		{ID: "2", Name: "Read", Input: "ok"},
	}
	results, err := ExecuteCalls(context.Background(), calls, []Tool{tool}, SchedulerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if results[1].Err != nil {
		t.Fatalf("known tool should succeed, got %v", results[1].Err)
	}
}

func TestExecuteCalls_ContextCancellationStopsRemaining(t *testing.T) {
	t.Cleanup(featureflag.Reload)
	featureflag.Set(featureflag.FlagParallelTools, false) // serial → easier to observe
	tool := &fakeTool{name: "Slow", d: 200 * time.Millisecond, safe: false}
	calls := []Call{
		{ID: "1", Name: "Slow", Input: "a"},
		{ID: "2", Name: "Slow", Input: "b"},
		{ID: "3", Name: "Slow", Input: "c"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	results, _ := ExecuteCalls(ctx, calls, []Tool{tool}, SchedulerConfig{})
	cancelled := 0
	for _, r := range results {
		if errors.Is(r.Err, context.DeadlineExceeded) || errors.Is(r.Err, context.Canceled) {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Fatalf("expected at least one result to carry ctx error, got %+v", results)
	}
}

// Benchmark: 8 concurrent-safe tools × 100ms each.
// Serial baseline: 800ms · parallel target: ~100ms (8× speedup).
func BenchmarkParallelVsSerial(b *testing.B) {
	tool := &fakeTool{name: "Read", d: 100 * time.Millisecond, safe: true}
	calls := make([]Call, 8)
	for i := range calls {
		calls[i] = Call{ID: string(rune('a' + i)), Name: "Read", Input: "x"}
	}
	b.Run("parallel", func(b *testing.B) {
		featureflag.Set(featureflag.FlagParallelTools, true)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ExecuteCalls(context.Background(), calls, []Tool{tool}, SchedulerConfig{MaxParallel: 8})
		}
	})
	b.Run("serial", func(b *testing.B) {
		featureflag.Set(featureflag.FlagParallelTools, false)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ExecuteCalls(context.Background(), calls, []Tool{tool}, SchedulerConfig{MaxParallel: 8})
		}
	})
	b.Cleanup(featureflag.Reload)
}
