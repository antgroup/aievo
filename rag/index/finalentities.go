package index

import (
	"context"

	"github.com/antgroup/aievo/rag"
)

func FinalEntities(_ context.Context, args *rag.WorkflowContext) error {
	me := make(map[string]*rag.Entity)
	for _, entity := range args.Entities {
		me[entity.Title] = entity
	}
	for _, community := range args.Communities {
		if me[community.Title] == nil {
			continue
		}
		me[community.Title].Communities = append(me[community.Title].Communities,
			community.Community)
	}
	for _, relationship := range args.Relationships {
		relationship.Source.Degree++
		relationship.Target.Degree++
	}
	return nil
}
