package rag

import (
	"context"
)

// Retriever is the interface for creating vector embeddings from texts.
type Retriever interface {
	// Query 根据一组文本，查询最相关的limit文本
	Query(ctx context.Context, text string, source []string, limit int) ([]string, error)
}
