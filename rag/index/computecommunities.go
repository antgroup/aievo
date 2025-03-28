package index

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/antgroup/aievo/rag"
)

// ComputeCommunities 通过莱顿算法，将整个社区进行划分
func ComputeCommunities(ctx context.Context, args *rag.WorkflowContext) error {
	communities, err := leidenCommunities(ctx, args)
	if err != nil {
		return err
	}
	// id 复用 entity 的 id
	m := make(map[string]string)
	for _, entity := range args.Entities {
		m[entity.Title] = entity.Id
	}
	for _, cluster := range communities {
		args.Communities = append(args.Communities,
			&rag.Community{
				Id:              m[cluster.Node],
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
func leidenCommunities(ctx context.Context, args *rag.WorkflowContext) ([]*rag.HierarchicalCluster, error) {
	{
		type Relation struct {
			Source string  `json:"source"`
			Target string  `json:"target"`
			Weight float64 `json:"weight"`
		}
		relations := make([]*Relation, 0, 100)
		for _, relationship := range args.Relationships {
			relations = append(relations, &Relation{
				Source: relationship.Source.Title,
				Target: relationship.Target.Title,
				Weight: relationship.Weight,
			})
		}

		pythonPath, err := getPythonPath()
		if err != nil {
			return nil, err
		}
		marshal, _ := json.Marshal(relations)

		temp, err := os.CreateTemp(os.TempDir(), "*.json")
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
		}()

		_, _ = temp.Write(marshal)

		output, err := exec.Command(pythonPath,
			filepath.Join(getAIevoPath(), "rag/index/leiden.py"),
			"--input", temp.Name()).Output()
		if err != nil {
			return nil, err
		}
		clusters := make([]*rag.HierarchicalCluster, 0, len(relations))
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

func getAIevoPath() string {
	aievoPath := os.Getenv("AIEVO_PATH")
	if aievoPath == "" {
		aievoPath, _ = os.Getwd()
		split := strings.Split(aievoPath, "aievo")
		aievoPath = filepath.Join(split[0], "aievo")
	}
	return aievoPath
}

func getPythonPath() (string, error) {
	pythonPath := os.Getenv("PYTHON_PATH")
	if pythonPath == "" {
		output, err := exec.Command("which", "python").Output()
		if err != nil {
			return "", err
		}
		pythonPath = string(output)
	}
	return strings.TrimSpace(pythonPath), nil
}
