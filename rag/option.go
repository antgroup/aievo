package rag

import (
	"github.com/antgroup/aievo/llm"
	"gorm.io/gorm"
)

type Option func(c *WorkflowConfig)

func WithChunkSize(size int) Option {
	return func(c *WorkflowConfig) {
		c.ChunkSize = size
	}
}

func WithChunkOverlap(overlap int) Option {
	return func(c *WorkflowConfig) {
		c.ChunkOverlap = overlap
	}
}

func WithSeparators(separators []string) Option {
	return func(c *WorkflowConfig) {
		c.Separators = separators
	}
}

func WithMaxToken(token int) Option {
	return func(c *WorkflowConfig) {
		c.MaxToken = token
	}
}

func WithMaxTurn(turn int) Option {
	return func(c *WorkflowConfig) {
		c.MaxTurn = turn
	}
}

func WithEntityTypes(types []string) Option {
	return func(c *WorkflowConfig) {
		c.EntityTypes = types
	}
}

func WithLLM(LLM llm.LLM) Option {
	return func(c *WorkflowConfig) {
		c.LLM = LLM
	}
}

func WithLLMCallConcurrency(concurrency int) Option {
	return func(c *WorkflowConfig) {
		c.LLMCallConcurrency = concurrency
	}
}

func WithDB(db *gorm.DB) Option {
	return func(c *WorkflowConfig) {
		c.DB = db
	}
}

func WithCacheDir(cacheDir string) Option {
	return func(c *WorkflowConfig) {
		c.CacheDir = cacheDir
	}
}

type QueryOption func(c *QueryConfig)

func WithQueryMaxToken(maxToken int) QueryOption {
	return func(c *QueryConfig) {
		c.LLMMaxToken = maxToken
	}
}

func WithQueryLLM(LLM llm.LLM) QueryOption {
	return func(c *QueryConfig) {
		c.LLM = LLM
	}
}

func WithRetriever(retriever Retriever) QueryOption {
	return func(c *QueryConfig) {
		c.Retriever = retriever
	}
}

func WithQueryMaxTurn(turn int) QueryOption {
	return func(c *QueryConfig) {
		c.MaxTurn = turn
	}
}
