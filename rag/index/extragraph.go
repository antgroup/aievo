package index

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag/index/prompts"
	"github.com/antgroup/aievo/utils/parallel"
	"github.com/thoas/go-funk"
)

var (
	_completionDelimiter = "<|COMPLETE|>"
	_tupleDelimiter      = "<|>"
	_recordDelimiter     = "##"
)

func ExtraGraph(ctx context.Context, args *WorkflowContext) error {
	err := extractEntities(ctx, args)
	if err != nil {
		return err
	}
	// 当前是 按照 title-type 进行聚类的，去除重复的，仅保留一个title
	m := make(map[string]*Entity)
	entities := make([]*Entity, 0, len(args.Entities))
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
			&Relationship{
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

func extractEntities(ctx context.Context, args *WorkflowContext) error {
	results := make([]string, len(args.TextUnits))
	parallel.Parallel(func(i int) any {
		template, _ := prompt.NewPromptTemplate(prompts.ExtraGraph)
		p, err := template.Format(map[string]any{
			"entity_types":         strings.Join(args.config.EntityTypes, ","),
			"tuple_delimiter":      _tupleDelimiter,
			"record_delimiter":     _recordDelimiter,
			"completion_delimiter": _completionDelimiter,
			"input_text":           args.TextUnits[i].Text,
		})
		if err != nil {
			panic(err)
		}
		if p == "" {
			fmt.Println("here")
		}
		for num := 0; num < 3; num++ {
			result, err := args.config.LLM.GenerateContent(ctx,
				[]llm.Message{llm.NewUserMessage("", p)},
				llm.WithTemperature(0.1))
			if err == nil {
				results[i] = result.Content
				break
			}
		}
		if results[i] == "" {
			return ""
		}
		for num := 0; num < 3; num++ {
			result, err := args.config.LLM.GenerateContent(ctx,
				[]llm.Message{
					llm.NewUserMessage("", p),
					llm.NewAssistantMessage("", results[i], nil),
					llm.NewUserMessage("", prompts.ContinueExtra),
				},
				llm.WithTemperature(0.1))
			if err == nil {
				results[i] = strings.TrimSpace(results[i] + result.Content)
				return ""
			}
		}
		return ""
	}, len(args.TextUnits), args.config.LLMCallConcurrency)

	// 将结果解析成 graph 和 relationship
	return parseResults(ctx, args, results)
}

func summaryDesc(ctx context.Context, args *WorkflowContext) error {
	template, _ := prompt.NewPromptTemplate(prompts.SummarizeDescription)
	// 进一步总结entity desc
	parallel.Parallel(func(i int) any {
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
			result, err := args.config.LLM.GenerateContent(ctx,
				[]llm.Message{llm.NewUserMessage("", p)},
				llm.WithTemperature(0.1))
			if err == nil {
				args.Entities[i].Desc = result.Content
				return nil
			}
		}
		return nil
	}, len(args.Entities), args.config.LLMCallConcurrency)

	// 进一步总结 relation desc
	parallel.Parallel(func(i int) any {
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
			result, err := args.config.LLM.GenerateContent(ctx,
				[]llm.Message{llm.NewUserMessage("", p)},
				llm.WithTemperature(0.1))
			if err == nil {
				args.TmpRelationships[i].Desc = result.Content
				return nil
			}
		}
		return nil
	}, len(args.TmpRelationships), args.config.LLMCallConcurrency)
	return nil
}

func parseResults(_ context.Context, args *WorkflowContext, results []string) error {
	entities := make([]*Entity, 0, len(results))
	relations := make([]*TmpRelationship, 0, len(results))
	// 用于合并相同的信息
	eMap := make(map[string]*Entity)
	rMap := make(map[string]*TmpRelationship)

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
				entity := &Entity{
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
						entity := &Entity{
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

				relation := &TmpRelationship{
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
