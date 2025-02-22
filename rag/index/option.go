package index

import (
	"github.com/antgroup/aievo/llm"
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
