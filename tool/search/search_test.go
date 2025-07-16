package search

import (
	"context"
	"os"
	"testing"
)

// const apiKey = "56c149a2cb136097da7d2383ed0b7652c13414d0f9e341bf03e6d67fc2820bb2"
var apiKey = os.Getenv("SERPAPI_API_KEY")

func TestGoogleSearch(t *testing.T) {
	tool, _ := New(
		WithEngine("google"),
		WithApiKey(apiKey),
		WithTopK(3),
	)
	ret, err := tool.Call(context.Background(), `{
	"query": "the best soccer player in history, American president's wife, the capital of France",
}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)
}

func TestBingSearch(t *testing.T) {
	tool, _ := New(
		WithEngine("bing"),
		WithApiKey(apiKey),
		WithTopK(10),
	)
	ret, err := tool.Call(context.Background(), `{
	"query": "ai"
}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)
}

func TestBaiduSearch(t *testing.T) {
	tool, _ := New(
		WithEngine("baidu"),
		WithApiKey(apiKey),
		WithTopK(10),
	)
	ret, err := tool.Call(context.Background(), `{
	"query": "ai"
}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(ret)
}
