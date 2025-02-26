package index

import (
	"context"
)

func FinalNodes(ctx context.Context, args *WorkflowContext) error {
	m := make(map[*Entity]int)
	for _, relation := range args.Relationships {
		m[relation.Source]++
		m[relation.Target]++
	}
	mEntities := make(map[string]int)
	for i, entity := range args.Entities {
		mEntities[entity.Title] = i
	}
	for _, community := range args.Communities {
		entity := args.Entities[mEntities[community.Title]]
		args.Nodes = append(args.Nodes,
			&Node{
				Id:        entity.Id,
				Title:     entity.Title,
				Community: community.Community,
				Level:     community.Level,
				Degree:    m[entity],
			})
	}
	return nil
}
