package index

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/antgroup/aievo/rag"
)

var (
	_defaultMaxToken    = 12000
	_defaultEntityTypes = []string{
		"person", "organization", "geo", "event"}
	_defaultLLMConcurrency = 10
)

type Workflow struct {
	nodes  []rag.Progress
	config *rag.WorkflowConfig
}

// NewWorkflow 初始化一个 index workflow, 在最后可以加一个存储的 progress，便于将数据存储到数据库
func NewWorkflow(nodes []rag.Progress, opts ...Option) (*Workflow, error) {
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

func DefaultNodes() []rag.Progress {
	return []rag.Progress{
		BaseDocuments,
		BaseTextUnits,
		FinalDocuments,
		ExtraGraph,
		ComputeCommunities,
		FinalEntities,
		FinalNodes,
		FinalCommunities,
		FinalTextUnits,
		FinalCommunityReport,
	}
}

func Default() (*Workflow, error) {
	return NewWorkflow(
		DefaultNodes(),
		WithMaxToken(_defaultMaxToken),
		WithEntityTypes(_defaultEntityTypes),
		WithLLMCallConcurrency(_defaultLLMConcurrency))
}

func (w *Workflow) Run(ctx context.Context, filepath string) error {
	args := &rag.WorkflowContext{
		BasePath: filepath,
		Config:   w.config,
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
	hash.Write([]byte(strings.ToUpper(s)))
	return hex.EncodeToString(hash.Sum(nil))
}
