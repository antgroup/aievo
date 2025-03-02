package index

import (
	"context"
)

func FinalTextUnits(ctx context.Context, args *WorkflowContext) error {
	me2t := make(map[string][]string)
	mr2t := make(map[string][]string)
	for _, entity := range args.Entities {
		for _, unitId := range entity.TextUnitIds {
			me2t[unitId] = append(me2t[unitId],
				entity.Id)
		}
	}

	for _, r := range args.Relationships {
		for _, unitId := range r.TextUnitIds {
			mr2t[unitId] = append(mr2t[unitId], r.Id)
		}
	}

	for _, unit := range args.TextUnits {
		unit.EntityIds = me2t[unit.Id]
		unit.RelationshipIds = mr2t[unit.Id]
	}

	return nil
}
