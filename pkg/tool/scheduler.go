package tool

import (
	"context"
	"runtime"
	"sync"

	"golang.org/x/sync/errgroup"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/featureflag"
)

// SchedulerConfig tunes ExecuteCalls.
type SchedulerConfig struct {
	// MaxParallel bounds concurrent invocations of concurrency-safe tools.
	// Zero means runtime.GOMAXPROCS(0).
	MaxParallel int
}

// ExecuteCalls runs `calls` against `tools`, partitioning by IsConcurrencySafe.
//
// Returned []Result is in *model order* (matching calls), regardless of
// completion order — this stable ordering matters because LLMs feed the
// tool results back in the same sequence as the requests.
//
// Errors are captured per-call in Result.Err. The function itself returns
// only ctx.Err() when the context is cancelled; otherwise nil. The caller
// inspects Results to decide retry / surfacing.
func ExecuteCalls(ctx context.Context, calls []Call, tools []Tool, cfg SchedulerConfig) ([]Result, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = runtime.GOMAXPROCS(0)
	}
	parallel := featureflag.Enabled(featureflag.FlagParallelTools)

	results := make([]Result, len(calls))
	// Resolve tools once; missing tools fail fast and never enter the executor.
	resolved := make([]Tool, len(calls))
	for i, c := range calls {
		t := FindByName(c.Name, tools)
		results[i].CallID = c.ID
		results[i].Name = c.Name
		if t == nil {
			results[i].Err = aierrs.NewFatal(unknownTool(c.Name))
			continue
		}
		resolved[i] = t
	}

	// Decide partitioning. When PARALLEL_TOOLS is disabled, treat every call
	// as unsafe → matches aievo's serial-only behaviour exactly.
	safeIdx := make([]int, 0, len(calls))
	unsafeIdx := make([]int, 0, len(calls))
	for i, t := range resolved {
		if t == nil || results[i].Err != nil {
			continue
		}
		if parallel && t.IsConcurrencySafe() {
			safeIdx = append(safeIdx, i)
		} else {
			unsafeIdx = append(unsafeIdx, i)
		}
	}

	// Phase 1: safe calls in parallel, bounded by cfg.MaxParallel.
	if len(safeIdx) > 0 {
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(cfg.MaxParallel)
		var mu sync.Mutex // protects results writes from the parallel workers
		for _, i := range safeIdx {
			i := i
			g.Go(func() error {
				if err := gctx.Err(); err != nil {
					mu.Lock()
					results[i].Err = err
					mu.Unlock()
					return nil // continue draining other goroutines
				}
				out, err := resolved[i].Call(gctx, calls[i].Input)
				mu.Lock()
				results[i].Output = out
				results[i].Err = err
				mu.Unlock()
				return nil
			})
		}
		_ = g.Wait()
	}

	// Phase 2: unsafe calls strictly sequential, honouring ctx per call.
	for _, i := range unsafeIdx {
		if err := ctx.Err(); err != nil {
			results[i].Err = err
			continue
		}
		out, err := resolved[i].Call(ctx, calls[i].Input)
		results[i].Output = out
		results[i].Err = err
	}

	return results, ctx.Err()
}

// unknownTool produces a clear error for hallucinated tool names.
type unknownToolErr struct{ Name string }

func (e *unknownToolErr) Error() string { return "tool not found: " + e.Name }
func unknownTool(name string) error     { return &unknownToolErr{Name: name} }
