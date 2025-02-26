package index

import (
	"context"
)

func FinalEntities(ctx context.Context, args *WorkflowContext) error {
	for i, entity := range args.Entities {
		// for computer community
		entity.Index = int64(i)
	}
	return nil
}
