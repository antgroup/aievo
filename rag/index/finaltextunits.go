package index

import (
	"context"

	"github.com/thoas/go-funk"
)

func FinalTextUnits(ctx context.Context, args *WorkflowContext) error {
	me2t := make(map[string][]string)

	for _, entity := range args.Entities {
		for _, unitId := range entity.TextUnitIds {
			me2t[unitId] = append(me2t[unitId],
				entity.Id)
		}
	}

	for _, unit := range args.TextUnits {
		relations := make([]string, 0, 20)
		entityIds := me2t[unit.Id]
		for _, relationship := range args.Relationships {
			if funk.ContainsString(entityIds, relationship.Source.Id) &&
				funk.ContainsString(entityIds, relationship.Target.Id) {
				relations = append(relations, relationship.Id)
			}
		}
		unit.RelationshipIds = funk.UniqString(relations)
		unit.EntityIds = entityIds
	}

	return nil
}
