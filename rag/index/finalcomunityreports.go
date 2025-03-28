package index

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/rag/prompts"
	"github.com/antgroup/aievo/utils/counter"
	"github.com/antgroup/aievo/utils/json"
	"github.com/antgroup/aievo/utils/parallel"
	"github.com/pkoukk/tiktoken-go"
)

func FinalCommunityReport(ctx context.Context, args *rag.WorkflowContext) error {
	mr := make(map[string]*rag.Relationship)
	reports := make(map[int]*rag.Report)
	reportsMutex := &sync.Mutex{}
	template, _ := prompt.NewPromptTemplate(prompts.SummarizeCommunity)

	for _, r := range args.Relationships {
		mr[r.Id] = r
	}

	maxLevel := -1
	mlevel := make(map[int][]*rag.Community)
	for _, community := range args.Communities {
		if community.Level > maxLevel {
			maxLevel = community.Level
		}
		mlevel[community.Level] = append(mlevel[community.Level], community)
	}

	total := 0
	for i := 0; i <= maxLevel; i++ {
		total += len(mlevel[i])
	}

	c := counter.NewCounter(
		counter.WithTotal(total),
		counter.WithDesc("final community report"),
	)

	// args.Communities 已经按照level进行过排序，同一level的可以并发执行
	for i := 0; i <= maxLevel; i++ {
		parallel.Parallel(func(idx int) any {
			defer c.Add()
			rs := make([]*rag.Relationship, 0, len(mlevel[i][idx].RelationshipIds))
			for _, r := range mlevel[i][idx].RelationshipIds {
				if mr[r] != nil {
					rs = append(rs, mr[r])
				}
			}
			content := buildCommunityReportContext(
				ctx, rs, nil, args.Config.MaxToken)
			p, _ := template.Format(map[string]any{
				"input_text": content,
			})
			for num := 0; num < 3; num++ {
				llmMsgs := []llm.Message{llm.NewUserMessage("", p)}
				result, err := CallModel(ctx, args, llmMsgs, num == 0)
				if err == nil {
					report := &rag.Report{}
					err = json.Unmarshal([]byte(json.TrimJsonString(result.Content)), report)
					if err != nil {
						continue
					}
					if mlevel[i][idx] != nil {
						reportsMutex.Lock()
						reports[mlevel[i][idx].Community] = report
						reportsMutex.Unlock()
					}
					return ""
				}
			}
			fmt.Println("[WARN] generate community report failed")
			return ""
		}, len(mlevel[i]), args.Config.LLMCallConcurrency)
	}
	for community, report := range reports {
		report.Community = community
		args.Reports = append(args.Reports, report)
	}
	sort.Slice(args.Reports, func(i, j int) bool {
		return args.Reports[i].Community < args.Reports[j].Community
	})
	return nil
}

// 从level开始倒序处理，level越高，社区越小，越细化
// 对于level较低的社区，社区数量比较大，
// todo: 是否需要考虑超出max token 时，用子报告替换部分relations
func buildCommunityReportContext(ctx context.Context,
	edges []*rag.Relationship, communities []*rag.Community, maxToken int) string {
	sort.Slice(communities, func(i, j int) bool {
		return communities[i].Size > communities[j].Size
	})
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].CombinedDegree > edges[j].CombinedDegree
	})
	tk, _ := tiktoken.GetEncoding(rag.DefaultTokenEncoding)

	entityContent := "-----Entities-----\ntitle,description,degree\n"
	edgeContent := "-----Relationships-----\nsource,target,description,combined_degree\n"
	me := make(map[string]struct{})

	for i := 0; i < len(edges) &&
		len(tk.Encode(edgeContent, nil, nil))+
			len(tk.Encode(entityContent, nil, nil)) <
			maxToken; i++ {
		if _, ok := me[edges[i].Source.Title]; !ok {
			me[edges[i].Source.Title] = struct{}{}
			entityContent += edges[i].Source.Title + "," + edges[i].Source.Desc + "," + strconv.Itoa(edges[i].Source.Degree) + "\n"
		}
		if _, ok := me[edges[i].Target.Title]; !ok {
			me[edges[i].Target.Title] = struct{}{}
			entityContent += edges[i].Target.Title + "," + edges[i].Target.Desc + "," + strconv.Itoa(edges[i].Target.Degree) + "\n"
		}
		edgeContent += edges[i].Source.Title + "," + edges[i].Target.Title + "," + edges[i].Desc + "," + strconv.Itoa(edges[i].CombinedDegree) + "\n"
	}

	return entityContent + "\n" + edgeContent

}
