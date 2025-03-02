package index

import (
	"context"
	"sort"
	"strconv"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag/index/prompts"
	"github.com/antgroup/aievo/rag/index/textsplitter"
	"github.com/antgroup/aievo/utils/json"
	"github.com/antgroup/aievo/utils/parallel"
	"github.com/pkoukk/tiktoken-go"
)

func FinalCommunityReport(ctx context.Context, args *WorkflowContext) error {
	mr := make(map[string]*Relationship)
	reports := make(map[int]*Report)
	template, _ := prompt.NewPromptTemplate(prompts.SummarizeCommunity)

	for _, r := range args.Relationships {
		mr[r.Id] = r
	}

	maxLevel := -1
	mlevel := make(map[int][]*Community)
	for _, community := range args.Communities {
		if community.Level > maxLevel {
			maxLevel = community.Level
		}
		mlevel[community.Level] = append(mlevel[community.Level], community)
	}

	// args.Communities 已经按照level进行过排序，同一level的可以并发执行
	for i := 0; i <= maxLevel; i++ {
		parallel.Parallel(func(idx int) any {
			rs := make([]*Relationship, 0, len(mlevel[i][idx].RelationshipIds))
			for _, r := range mlevel[i][idx].RelationshipIds {
				rs = append(rs, mr[r])
			}
			content := buildCommunityReportContext(
				ctx, rs, nil, args.config.MaxToken)
			p, _ := template.Format(map[string]any{
				"input_text": content,
			})
			for num := 0; num < 3; num++ {
				result, err := args.config.LLM.GenerateContent(ctx,
					[]llm.Message{llm.NewUserMessage("", p)},
					llm.WithTemperature(0.1))
				if err == nil {
					report := &Report{}
					err = json.Unmarshal(
						[]byte(json.TrimJsonString(result.Content)), report)
					if err != nil {
						continue
					}
					reports[mlevel[i][idx].Community] = report
					return ""
				}
			}
			return ""
		}, len(mlevel[i]), args.config.LLMCallConcurrency)
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
	edges []*Relationship, communities []*Community, maxToken int) string {
	sort.Slice(communities, func(i, j int) bool {
		return communities[i].Size > communities[j].Size
	})
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].CombinedDegree > edges[j].CombinedDegree
	})
	tk, _ := tiktoken.GetEncoding(textsplitter.DefaultTokenEncoding)

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
