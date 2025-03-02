package index

import (
	"context"
)

func FinalEntities(_ context.Context, args *WorkflowContext) error {
	me := make(map[string]*Entity)
	for _, entity := range args.Entities {
		me[entity.Title] = entity
	}
	for _, community := range args.Communities {
		me[community.Title].Communities = append(me[community.Title].Communities,
			community.Community)
	}
	for _, relationship := range args.Relationships {
		relationship.Source.Degree++
		relationship.Target.Degree++
	}
	return nil
}
