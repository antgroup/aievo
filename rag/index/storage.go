package index

import (
	"context"

	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/rag/storage/db"
)

func Load(ctx context.Context, wfCtx *rag.WorkflowContext) error {
	if wfCtx.Config.DB == nil {
		return nil
	}

	storage := db.NewStorage(db.WithDB(wfCtx.Config.DB))

	return storage.Load(ctx, wfCtx)
}

func Save(ctx context.Context, wfCtx *rag.WorkflowContext, indexProgress int) error {
	if wfCtx.Config.DB == nil {
		return nil
	}

	storage := db.NewStorage(db.WithDB(wfCtx.Config.DB))

	return storage.Save(ctx, wfCtx, indexProgress)
}
