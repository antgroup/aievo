package wikipedia

import (
	"context"
	"testing"
)

func TestWikipedia(t *testing.T) {
	t.Parallel()
	tool, _ := New()
	result, err := tool.Call(context.Background(), "{\"query\": \"ai\"")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("result: %s", result)
}
