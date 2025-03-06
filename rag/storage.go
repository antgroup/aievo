package rag

import (
	"context"
)

type Storage interface {
	// Load loads the workflow context from the storage
	Load(ctx context.Context, wfCtx *WorkflowContext) error
	// Save saves the workflow context to the storage
	Save(ctx context.Context, wfCtx *WorkflowContext) error
}
