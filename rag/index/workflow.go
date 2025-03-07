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
	DefaultMaxToken    = 1024 * 12
	DefaultMaxTurn     = 6
	DefaultEntityTypes = []string{
		"person", "organization", "geo", "event"}
	DefaultLLMConcurrency = 6
)

type Workflow struct {
	nodes  []rag.Progress
	config *rag.WorkflowConfig
}

// NewWorkflow 初始化一个 index workflow, 在最后可以加一个存储的 progress，便于将数据存储到数据库
func NewWorkflow(nodes []rag.Progress, opts ...rag.Option) (*Workflow, error) {
	w := &Workflow{
		nodes:  nodes,
		config: &rag.WorkflowConfig{},
	}
	for _, opt := range opts {
		opt(w.config)
	}
	if w.config.LLM == nil {
		return nil, errors.New("LLM is required")
	}
	if w.config.LLMCallConcurrency <= 0 {
		w.config.LLMCallConcurrency = DefaultLLMConcurrency
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
		SaveToStorage,
	}
}

func Default() (*Workflow, error) {
	return NewWorkflow(
		DefaultNodes(),
		rag.WithMaxToken(DefaultMaxToken),
		rag.WithEntityTypes(DefaultEntityTypes),
		rag.WithLLMCallConcurrency(DefaultLLMConcurrency))
}

func (w *Workflow) Run(ctx context.Context, wfCtx *rag.WorkflowContext) error {
	wfCtx.Config = w.config

	for _, process := range w.nodes {
		err := process(ctx, wfCtx)
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
