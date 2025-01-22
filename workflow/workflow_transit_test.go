package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var TestTransitWorkerSleep100MilliSecondsWorker = func(ctx context.Context, a ...any) (any, error) {
	time.Sleep(100 * time.Millisecond)
	return a[0], nil
}
var TestTransitWorkerSleep100MilliSecondsWorker2 = func(ctx context.Context, a ...any) (any, error) {
	time.Sleep(100 * time.Millisecond)
	return a[0], nil
}

func TestNewTransitAndRun(t *testing.T) {
	t.Run("run sync", func(t *testing.T) {
		transit := NewTransit("test", WithWorker(TestTransitWorkerSleep100MilliSecondsWorker))
		done := make(chan struct{})
		go func() {
			result, err := transit.Run(context.Background(), "test")
			assert.Equal(t, "test", result)
			assert.Nil(t, err)
			done <- struct{}{}
		}()
		<-done
	})
}
