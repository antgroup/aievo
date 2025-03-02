package index

import (
	"context"
	"sort"

	"github.com/antgroup/aievo/rag"
)

func FinalNodes(ctx context.Context, args *rag.WorkflowContext) error {
	m := make(map[*rag.Entity]int)
	for _, relation := range args.Relationships {
		m[relation.Source]++
		m[relation.Target]++
	}
	mEntities := make(map[string]int)
	mFlag := make(map[string]struct{})
	for i, entity := range args.Entities {
		mEntities[entity.Title] = i
		mFlag[entity.Title] = struct{}{}
	}
	for _, community := range args.Communities {
		entity := args.Entities[mEntities[community.Title]]
		delete(mFlag, community.Title)
		args.Nodes = append(args.Nodes,
			&rag.Node{
				Id:        entity.Id,
				Title:     entity.Title,
				Community: community.Community,
				Level:     community.Level,
				Degree:    m[entity],
			})
	}
	// 补充未用的节点
	for title := range mFlag {
		entity := args.Entities[mEntities[title]]
		args.Nodes = append(args.Nodes,
			&rag.Node{
				Id:        entity.Id,
				Title:     entity.Title,
				Community: -1,
				Level:     0,
				Degree:    m[entity],
			})
	}
	sort.Slice(args.Nodes, func(i, j int) bool {
		return args.Nodes[i].Level < args.Nodes[j].Level ||
			args.Nodes[i].Level == args.Nodes[j].Level &&
				args.Nodes[i].Community < args.Nodes[j].Community ||
			args.Nodes[i].Level == args.Nodes[j].Level &&
				args.Nodes[i].Community == args.Nodes[j].Community &&
				args.Nodes[i].Degree < args.Nodes[j].Degree

	})
	return nil
}
