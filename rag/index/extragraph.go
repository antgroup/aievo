package index

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag"
	prompts2 "github.com/antgroup/aievo/rag/prompts"
	"github.com/antgroup/aievo/utils/counter"
	"github.com/antgroup/aievo/utils/parallel"
	"github.com/antgroup/aievo/utils/ratelimit"
	"github.com/thoas/go-funk"
)

var (
	_completionDelimiter = "<|COMPLETE|>"
	_tupleDelimiter      = "<|>"
	_recordDelimiter     = "##"
)

func ExtraGraph(ctx context.Context, args *rag.WorkflowContext) error {
	err := extractEntities(ctx, args)
	if err != nil {
		return err
	}
	// 当前是 按照 title-type 进行聚类的，去除重复的，仅保留一个title
	m := make(map[string]*rag.Entity)
	entities := make([]*rag.Entity, 0, len(args.Entities))
	for _, entity := range args.Entities {
		if _, ok := m[entity.Title]; !ok {
			entities = append(entities, entity)
			m[entity.Title] = entity
			entity.TextUnitIds = funk.UniqString(entity.TextUnitIds)
		}
	}
	args.Entities = entities

	err = summaryDesc(ctx, args)
	if err != nil {
		return err
	}

	// 修复relation
	for _, relationship := range args.TmpRelationships {
		args.Relationships = append(args.Relationships,
			&rag.Relationship{
				Id:             relationship.Id,
				Source:         m[relationship.Source],
				Target:         m[relationship.Target],
				Desc:           relationship.Desc,
				Weight:         relationship.Weight,
				CombinedDegree: relationship.CombinedDegree,
				TextUnitIds:    funk.UniqString(relationship.TextUnitIds),
			})
	}

	return nil
}

func extractEntities(ctx context.Context, args *rag.WorkflowContext) error {
	c := counter.NewCounter(
		counter.WithTotal(len(args.TextUnits)),
		counter.WithDesc("extract entities"),
	)

	// 创建令牌桶限流器，每秒生成2个令牌，最大容量为10
	tb := ratelimit.NewTokenBucket(2, 10)

	results := make([]string, len(args.TextUnits))

	parallel.Parallel(func(i int) any {
		defer c.Add()
		template, _ := prompt.NewPromptTemplate(prompts2.ExtraGraph)
		p, err := template.Format(map[string]any{
			"entity_types":         strings.Join(args.Config.EntityTypes, ","),
			"tuple_delimiter":      _tupleDelimiter,
			"record_delimiter":     _recordDelimiter,
			"completion_delimiter": _completionDelimiter,
			"input_text":           args.TextUnits[i].Text,
		})
		if err != nil {
			panic(err)
		}
		for num := 0; num < args.Config.MaxTurn; num++ {
			// 等待获取令牌
			if err := tb.Wait(ctx); err != nil {
				fmt.Println("[WARN] get token failed:", err)
				continue
			}
			result, err := args.Config.LLM.GenerateContent(ctx,
				[]llm.Message{llm.NewUserMessage("", p)},
				llm.WithTemperature(0.1))
			if err == nil && result.Content != "" {
				results[i] = result.Content
				break
			}
		}
		if results[i] == "" {
			fmt.Println("[WARN] extract graph failed")
			return ""
		}
		for num := 0; num < args.Config.MaxTurn; num++ {
			// 等待获取令牌
			if err := tb.Wait(ctx); err != nil {
				fmt.Println("[WARN] get token failed:", err)
				continue
			}
			result, err := args.Config.LLM.GenerateContent(ctx,
				[]llm.Message{
					llm.NewUserMessage("", p),
					llm.NewAssistantMessage("", results[i], nil),
					llm.NewUserMessage("", prompts2.ContinueExtra),
				},
				llm.WithTemperature(0.1))
			if err == nil && result.Content != "" {
				results[i] = strings.TrimSpace(results[i] + result.Content)
				return ""
			}
		}
		fmt.Println("[WARN] continue extra failed")
		return ""
	}, len(args.TextUnits), args.Config.LLMCallConcurrency)

	// 将结果解析成 graph 和 relationship
	return parseResults(ctx, args, results)
}

func summaryDesc(ctx context.Context, args *rag.WorkflowContext) error {
	// 创建令牌桶限流器，每秒生成2个令牌，最大容量为10
	tb := ratelimit.NewTokenBucket(2, 10)

	c1 := counter.NewCounter(
		counter.WithTotal(len(args.Entities)),
		counter.WithDesc("summary entity description"),
	)

	template, _ := prompt.NewPromptTemplate(prompts2.SummarizeDescription)
	// 进一步总结entity desc
	parallel.Parallel(func(i int) any {
		defer c1.Add()
		descs := funk.UniqString(args.Entities[i].TmpDesc)
		if len(descs) == 0 {
			return nil
		}
		if len(descs) == 1 {
			args.Entities[i].Desc = descs[0]
			return nil
		}
		desc, _ := json.Marshal(descs)
		title, _ := json.Marshal(args.Entities[i].Title)
		p, _ := template.Format(map[string]any{
			"entity_name":      string(title),
			"description_list": string(desc),
		})
		for num := 0; num < 3; num++ {
			// 等待获取令牌
			if err := tb.Wait(ctx); err != nil {
				fmt.Println("[WARN] get token failed:", err)
				continue
			}
			result, err := args.Config.LLM.GenerateContent(ctx,
				[]llm.Message{llm.NewUserMessage("", p)},
				llm.WithTemperature(0.1))
			if err == nil || result.Content != "" {
				args.Entities[i].Desc = result.Content
				return nil
			}
		}
		fmt.Println("[WARN] summary description failed")
		return nil
	}, len(args.Entities), args.Config.LLMCallConcurrency)

	c2 := counter.NewCounter(
		counter.WithTotal(len(args.TmpRelationships)),
		counter.WithDesc("relation entity description"),
	)

	// 进一步总结 relation desc
	parallel.Parallel(func(i int) any {
		defer c2.Add()
		descs := funk.UniqString(args.TmpRelationships[i].TmpDesc)
		if len(descs) == 0 {
			return nil
		}
		if len(descs) == 1 {
			args.TmpRelationships[i].Desc = descs[0]
			return nil
		}
		desc, _ := json.Marshal(descs)
		title, _ := json.Marshal([]string{
			args.TmpRelationships[i].Source,
			args.TmpRelationships[i].Target})
		p, _ := template.Format(map[string]any{
			"entity_name":      string(title),
			"description_list": string(desc),
		})
		for num := 0; num < 3; num++ {
			// 等待获取令牌
			if err := tb.Wait(ctx); err != nil {
				fmt.Println("[WARN] get token failed:", err)
				continue
			}
			result, err := args.Config.LLM.GenerateContent(ctx,
				[]llm.Message{llm.NewUserMessage("", p)},
				llm.WithTemperature(0.1))
			if err == nil || result.Content != "" {
				args.TmpRelationships[i].Desc = result.Content
				return nil
			}
		}
		fmt.Println("[WARN] continue summary description failed")
		return nil
	}, len(args.TmpRelationships), args.Config.LLMCallConcurrency)
	return nil
}

func parseResults(_ context.Context, args *rag.WorkflowContext, results []string) error {
	entities := make([]*rag.Entity, 0, len(results))
	relations := make([]*rag.TmpRelationship, 0, len(results))
	// 用于合并相同的信息
	eMap := make(map[string]*rag.Entity)
	rMap := make(map[string]*rag.TmpRelationship)

	// add entity
	for idx, result := range results {
		records := strings.Split(result, _recordDelimiter)
		mentityType := make(map[string]string)
		for _, record := range records {
			record = strings.TrimSpace(record)
			record = strings.TrimPrefix(record, "(")
			record = strings.TrimSuffix(record, ")")
			attrs := strings.Split(record, _tupleDelimiter)
			if len(attrs) >= 4 && attrs[0] == `"entity"` {
				title := strings.ToUpper(strings.Trim(attrs[1], `"`))
				typ := strings.ToUpper(strings.Trim(attrs[2], `"`))
				entity := &rag.Entity{
					Id:          id(title + _tupleDelimiter + typ),
					Title:       title,
					Type:        typ,
					TmpDesc:     []string{},
					TextUnitIds: []string{},
				}
				mentityType[entity.Title] = entity.Type
				if eMap[entity.Id] == nil {
					eMap[entity.Id] = entity
					entities = append(entities, eMap[entity.Id])
				}
				if strings.Trim(attrs[3], `"`) != "" {
					eMap[entity.Id].TmpDesc = append(
						eMap[entity.Id].TmpDesc, strings.Trim(attrs[3], `"`))
				}

				eMap[entity.Id].TextUnitIds = append(
					eMap[entity.Id].TextUnitIds, args.TextUnits[idx].Id)
			}
			if len(attrs) >= 5 && attrs[0] == `"relationship"` {
				weight, err := strconv.ParseFloat(strings.Trim(attrs[4], `"`), 64)
				if err != nil {
					weight = 1.0
				}
				source := strings.ToUpper(strings.Trim(attrs[1], `"`))
				target := strings.ToUpper(strings.Trim(attrs[2], `"`))
				desc := strings.Trim(attrs[3], `"`)
				titles := []string{source, target}
				for i, key := range []string{
					id(source + _tupleDelimiter + mentityType[source]),
					id(target + _tupleDelimiter + mentityType[target]),
				} {
					if eMap[key] == nil {
						// 添加 source
						entity := &rag.Entity{
							Id:          key,
							Title:       titles[i],
							Type:        "",
							TmpDesc:     []string{},
							TextUnitIds: []string{},
						}
						eMap[entity.Id] = entity
						entities = append(entities, entity)
					}
					eMap[key].TextUnitIds = append(eMap[key].TextUnitIds, args.TextUnits[idx].Id)
				}

				relation := &rag.TmpRelationship{
					Id:             id(source + _tupleDelimiter + target),
					Source:         source,
					Target:         target,
					Desc:           "",
					TmpDesc:        []string{},
					Weight:         0,
					CombinedDegree: 0,
					TextUnitIds:    []string{},
				}
				if rMap[relation.Id] == nil {
					rMap[relation.Id] = relation
					rMap[id(target+_tupleDelimiter+source)] = relation
					relations = append(relations, rMap[relation.Id])
				}
				rMap[relation.Id].TmpDesc = append(
					rMap[relation.Id].TmpDesc, desc)
				rMap[relation.Id].TextUnitIds = append(
					rMap[relation.Id].TextUnitIds, args.TextUnits[idx].Id)
				rMap[relation.Id].Weight += weight
			}
		}
	}

	args.Entities = entities
	args.TmpRelationships = relations
	return nil
}
