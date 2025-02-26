package index

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"time"
)

// ComputeCommunities 通过莱顿算法，将整个社区进行划分
func ComputeCommunities(ctx context.Context, args *WorkflowContext) error {
	communities, err := leidenCommunities(ctx, args)
	if err != nil {
		return err
	}
	for _, cluster := range communities {
		args.Communities = append(args.Communities,
			&Community{
				Id:              id(strconv.Itoa(cluster.Cluster)),
				Title:           cluster.Node,
				Community:       cluster.Cluster,
				Level:           cluster.Level,
				RelationshipIds: nil,
				TextUnitIds:     nil,
				Parent:          cluster.ParentCluster,
				EntityIds:       nil,
				Period:          time.Now().Format("2006-01-02"),
				Size:            0,
			})
	}
	sort.Slice(args.Communities, func(i, j int) bool {
		return args.Communities[i].Level > args.Communities[j].Level ||
			args.Communities[i].Level == args.Communities[j].Level &&
				args.Communities[i].Community > args.Communities[j].Community
	})
	return nil
}

// 使用leiden算法来计算社区分类
// todo: fix me
func leidenCommunities(ctx context.Context, args *WorkflowContext) ([]*HierarchicalCluster, error) {
	{
		// todo: fix me
		type Relation struct {
			Source string `json:"source"`
			Target string `json:"target"`
		}
		relations := make([]*Relation, 0, 100)
		for _, relationship := range args.Relationships {
			relations = append(relations, &Relation{
				Source: relationship.Source.Title,
				Target: relationship.Target.Title,
			})
		}

		marshal, _ := json.Marshal(relations)
		output, err := exec.Command(os.Getenv("PYTHON_PATH"),
			os.Getenv("AIEVO_PATH")+"rag/index/leiden.py",
			"--input", string(marshal)).Output()
		clusters := make([]*HierarchicalCluster, 0, len(relations))
		var result any
		err = json.Unmarshal(output, &result)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(output, &clusters)
		if err != nil {
			return nil, err
		}
		return clusters, nil
	}
}
