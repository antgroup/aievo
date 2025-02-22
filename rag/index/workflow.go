package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/antgroup/aievo/llm"
)

var (
	_defaultMaxToken    = 12000
	_defaultEntityTypes = []string{
		"person", "organization", "geo", "event"}
	_defaultLLMConcurrency = 10
)

type Workflow struct {
	nodes  []Progress
	config *WorkflowConfig
}

type WorkflowConfig struct {
	ChunkSize          int
	ChunkOverlap       int
	Separators         []string
	MaxToken           int
	EntityTypes        []string
	LLM                llm.LLM
	LLMCallConcurrency int
}

func NewWorkflow(nodes []Progress, opts ...Option) (*Workflow, error) {
	w := &Workflow{nodes: nodes}
	for _, opt := range opts {
		opt(w.config)
	}
	if w.config.LLM == nil {
		return nil, errors.New("LLM is required")
	}
	if w.config.LLMCallConcurrency <= 0 {
		w.config.LLMCallConcurrency = _defaultLLMConcurrency
	}
	return w, nil
}

func Default() (*Workflow, error) {
	return NewWorkflow(
		[]Progress{
			BaseDocuments,
			BaseTextUnits,
		},
		WithMaxToken(_defaultMaxToken),
		WithEntityTypes(_defaultEntityTypes),
		WithLLMCallConcurrency(_defaultLLMConcurrency))
}

func (w *Workflow) Run(ctx context.Context, filepath string) error {
	args := &WorkflowContext{
		basepath: filepath,
		config:   w.config,
	}

	for _, process := range w.nodes {
		err := process(ctx, args)
		if err != nil {
			return err
		}
	}
	return nil
}

func id(s string) string {
	hash := sha256.New()
	hash.Write([]byte(s))
	return hex.EncodeToString(hash.Sum(nil))
}
