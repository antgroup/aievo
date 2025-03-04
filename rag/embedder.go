package rag

import (
	"context"
)

// Embedder is the interface for creating vector embeddings from texts.
type Embedder interface {
	// EmbedQuery 根据一组文本，查询最相关的limit文本
	EmbedQuery(ctx context.Context, text string, source []string, limit int) ([]string, error)
}
