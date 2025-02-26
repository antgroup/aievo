package index

import (
	"context"
)

func FinalRelations(ctx context.Context, args *WorkflowContext) error {
	// 计算每个节点的边
	// 将source + target degree = relation degree
	m := make(map[*Entity]int)
	for _, relation := range args.Relationships {
		m[relation.Source]++
		m[relation.Target]++
	}
	for _, relation := range args.Relationships {
		relation.CombinedDegree = m[relation.Source] + m[relation.Target]
	}
	return nil
}
