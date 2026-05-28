package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"
)

func TestRetryable_True(t *testing.T) {
	if !IsRetryable(NewTransient(stderrors.New("net"))) {
		t.Fatal("transient should be retryable")
	}
	if !IsRetryable(NewLLMError("openai", 503, stderrors.New("upstream"))) {
		t.Fatal("5xx LLM should be retryable")
	}
	if !IsRetryable(NewLLMError("openai", 429, stderrors.New("rate"))) {
		t.Fatal("429 should be retryable")
	}
	if !IsRetryable(NewToolError("Bash", stderrors.New("io"), true)) {
		t.Fatal("ToolError with retry=true should be retryable")
	}
}

func TestRetryable_False(t *testing.T) {
	if IsRetryable(NewFatal(stderrors.New("bad"))) {
		t.Fatal("fatal should not be retryable")
	}
	if IsRetryable(NewLLMError("openai", 400, stderrors.New("bad req"))) {
		t.Fatal("400 should not be retryable")
	}
	if IsRetryable(stderrors.New("plain error")) {
		t.Fatal("plain error should not be retryable")
	}
}

func TestRetryable_WrapsUnwrap(t *testing.T) {
	inner := NewTransient(stderrors.New("x"))
	wrapped := fmt.Errorf("ctx: %w", inner)
	if !IsRetryable(wrapped) {
		t.Fatal("wrapped retryable should still be retryable")
	}
	var le *LLMError
	src := NewLLMError("p", 500, stderrors.New("oops"))
	wrapped2 := fmt.Errorf("higher: %w", src)
	if !stderrors.As(wrapped2, &le) {
		t.Fatal("errors.As should unwrap typed error")
	}
}

func TestRetry_SucceedsAfterTransient(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), Policy{
		MaxAttempts: 3,
		BaseBackoff: 1 * time.Millisecond,
		MaxBackoff:  5 * time.Millisecond,
	}, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return NewTransient(stderrors.New("flaky"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NonRetryableStopsImmediately(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), Policy{MaxAttempts: 5, BaseBackoff: 1 * time.Millisecond},
		func(ctx context.Context) error {
			attempts++
			return NewFatal(stderrors.New("nope"))
		})
	if err == nil || attempts != 1 {
		t.Fatalf("expected immediate fail, got err=%v attempts=%d", err, attempts)
	}
}

func TestRetry_RespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Retry(ctx, DefaultPolicy, func(ctx context.Context) error {
		return NewTransient(stderrors.New("x"))
	})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("expected ctx.Canceled, got %v", err)
	}
}
