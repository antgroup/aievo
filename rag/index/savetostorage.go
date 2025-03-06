package index

import (
	"context"

	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/rag/storage/db"
)

func SaveToStorage(ctx context.Context, wfCtx *rag.WorkflowContext) error {
	if wfCtx.Config.DB == nil {
		return nil
	}

	storage := db.NewStorage(db.WithDB(wfCtx.Config.DB))

	return storage.Save(ctx, wfCtx)
}
