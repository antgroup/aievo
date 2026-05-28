package benchmark

import (
	"context"
	"testing"
	"time"

	"github.com/samson-samson/aievo-next/pkg/featureflag"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// simulatedIOTool mimics a tool that does ~100ms of I/O. Concurrency-safe
// (read-only API call, file read, etc.) so the scheduler can parallelise.
type simulatedIOTool struct{ delay time.Duration }

func (simulatedIOTool) Name() string             { return "Read" }
func (simulatedIOTool) Description() string      { return "simulated I/O" }
func (simulatedIOTool) Schema() []byte           { return nil }
func (simulatedIOTool) IsConcurrencySafe() bool  { return true }
func (t simulatedIOTool) Call(ctx context.Context, _ string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(t.delay):
	}
	return "ok", nil
}

// BenchmarkAievoNext_ParallelTools: aievo-next with default FlagParallelTools=true.
// BenchmarkAievoNext_SerialTools:   aievo-next with FlagParallelTools=false
//                                   (equivalent to aievo's BaseAgent.doAction).
//
// On Apple M-series:
//   8 calls × 100ms parallel ≈ 100ms
//   8 calls × 100ms serial   ≈ 800ms  → 8× speed-up
func benchToolBatch(b *testing.B, parallel bool) {
	featureflag.Set(featureflag.FlagParallelTools, parallel)
	b.Cleanup(featureflag.Reload)

	t := simulatedIOTool{delay: 100 * time.Millisecond}
	calls := make([]tool.Call, 8)
	for i := range calls {
		calls[i] = tool.Call{ID: string(rune('a' + i)), Name: "Read", Input: "x"}
	}
	cfg := tool.SchedulerConfig{MaxParallel: 8}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tool.ExecuteCalls(context.Background(), calls, []tool.Tool{t}, cfg)
	}
}

func BenchmarkAievoNext_ParallelTools(b *testing.B) { benchToolBatch(b, true) }
func BenchmarkAievoNext_SerialTools(b *testing.B)   { benchToolBatch(b, false) }
