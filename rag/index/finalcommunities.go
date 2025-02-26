package index

import (
	"context"
	"strconv"

	"github.com/thoas/go-funk"
)

func FinalCommunities(ctx context.Context, args *WorkflowContext) error {
	communities := make([]*Community, 0, 20)
	mc2e := make(map[*Community][]string)

	mc2t := make(map[*Community][]string)
	me2e := make(map[string]*Entity)

	for _, entity := range args.Entities {
		me2e[entity.Id] = entity
	}

	for _, community := range args.Communities {
		// entity ids
		mc2e[community] = append(mc2e[community],
			community.Id)
		// TextUnitIds
		mc2t[community] = append(mc2t[community],
			me2e[community.Id].TextUnitIds...)
	}

	idx := 0
	for community, entities := range mc2e {
		communities = append(communities, &Community{
			Id:              id(strconv.Itoa(idx)),
			Title:           "Community " + strconv.Itoa(idx),
			Community:       community.Community,
			Level:           community.Level,
			RelationshipIds: make([]string, 0, 20),
			TextUnitIds:     funk.UniqString(mc2t[community]),
			Parent:          community.Parent,
			EntityIds:       funk.UniqString(entities),
			Period:          community.Period,
			Size:            len(entities),
		})
		idx++
	}
	for _, community := range communities {
		relations := make([]string, 0, 20)
		for _, relationship := range args.Relationships {
			if funk.ContainsString(community.EntityIds, relationship.Source.Title) &&
				funk.ContainsString(community.EntityIds, relationship.Target.Title) {
				relations = append(relations, relationship.Id)
			}
		}
		community.RelationshipIds = funk.UniqString(relations)
	}
	args.Communities = communities

	return nil
}
