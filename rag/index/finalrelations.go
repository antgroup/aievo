package index

import (
	"context"

	"github.com/antgroup/aievo/rag"
)

func FinalRelations(ctx context.Context, args *rag.WorkflowContext) error {
	// 计算每个节点的边
	// 将source + target degree = relation degree
	m := make(map[*rag.Entity]int)
	for _, relation := range args.Relationships {
		m[relation.Source]++
		m[relation.Target]++
	}
	for _, relation := range args.Relationships {
		relation.CombinedDegree = m[relation.Source] + m[relation.Target]
	}
	return nil
}
