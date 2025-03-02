package index

import (
	"context"
	"sort"
	"strconv"

	"github.com/antgroup/aievo/rag"
	"github.com/thoas/go-funk"
)

func FinalCommunities(ctx context.Context, args *rag.WorkflowContext) error {
	communities := make([]*rag.Community, 0, 20)
	mc2e := make(map[int]*rag.Community)
	me2e := make(map[string]*rag.Entity)
	mLevelEntity := make(map[int][]string)

	for _, entity := range args.Entities {
		me2e[entity.Id] = entity
	}
	maxLevel := -1
	for _, c := range args.Communities {
		if _, ok := mc2e[c.Community]; !ok {
			mc2e[c.Community] = &rag.Community{
				Id:              strconv.Itoa(c.Community),
				Title:           "Community " + strconv.Itoa(c.Community),
				Community:       c.Community,
				Level:           c.Level,
				RelationshipIds: make([]string, 0, 20),
				TextUnitIds:     make([]string, 0, 20),
				Parent:          c.Parent,
				EntityIds:       make([]string, 0, 20),
				Period:          c.Period,
				Size:            0,
			}
			communities = append(communities, mc2e[c.Community])
		}
		mc2e[c.Community].EntityIds = append(
			mc2e[c.Community].EntityIds, c.Id)
		if _, ok := mLevelEntity[c.Level]; !ok {
			mLevelEntity[c.Level] = make([]string, 0, 20)
		}
		mLevelEntity[c.Level] = append(mLevelEntity[c.Level],
			me2e[c.Id].Title)
		if maxLevel < c.Level {
			maxLevel = c.Level
		}
	}

	for _, c := range communities {
		relations := make([]string, 0, 20)
		textUnits := make([]string, 0, 20)
		for _, r := range args.Relationships {
			if funk.ContainsString(mLevelEntity[c.Level], r.Source.Title) &&
				funk.ContainsString(mLevelEntity[c.Level], r.Target.Title) &&
				funk.ContainsString(c.EntityIds, r.Source.Id) &&
				funk.ContainsString(c.EntityIds, r.Target.Id) {
				relations = append(relations, r.Id)
				textUnits = append(textUnits, r.TextUnitIds...)
			}
		}
		c.RelationshipIds = funk.UniqString(relations)
		c.TextUnitIds = funk.UniqString(textUnits)
		c.EntityIds = funk.UniqString(c.EntityIds)
		c.Size = len(c.EntityIds)
	}

	// 按层级和社区ID排序
	sort.Slice(communities, func(i, j int) bool {
		if communities[i].Level == communities[j].Level {
			return communities[i].Community < communities[j].Community
		}
		return communities[i].Level < communities[j].Level
	})

	args.Communities = communities
	return nil

}
