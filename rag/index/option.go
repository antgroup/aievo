package index

import (
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/rag"
)

type Option func(c *rag.WorkflowConfig)

func WithChunkSize(size int) Option {
	return func(c *rag.WorkflowConfig) {
		c.ChunkSize = size
	}
}

func WithChunkOverlap(overlap int) Option {
	return func(c *rag.WorkflowConfig) {
		c.ChunkOverlap = overlap
	}
}

func WithSeparators(separators []string) Option {
	return func(c *rag.WorkflowConfig) {
		c.Separators = separators
	}
}

func WithMaxToken(token int) Option {
	return func(c *rag.WorkflowConfig) {
		c.MaxToken = token
	}
}

func WithEntityTypes(types []string) Option {
	return func(c *rag.WorkflowConfig) {
		c.EntityTypes = types
	}
}

func WithLLM(LLM llm.LLM) Option {
	return func(c *rag.WorkflowConfig) {
		c.LLM = LLM
	}
}

func WithLLMCallConcurrency(concurrency int) Option {
	return func(c *rag.WorkflowConfig) {
		c.LLMCallConcurrency = concurrency
	}
}
