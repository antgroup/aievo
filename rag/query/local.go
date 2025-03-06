package query

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/rag/prompts"
	"github.com/pkoukk/tiktoken-go"
	"github.com/thoas/go-funk"
)

func (r *RAG) LocalQuery(ctx context.Context, query string, method Method) (string, error) {
	switch method {
	case Local:
		t, _ := prompt.NewPromptTemplate(prompts.LocalQuery)
		data, err := r.localQueryContext(ctx, query)
		if err != nil {
			return "", err
		}
		p, _ := t.Format(map[string]any{
			"data": data,
		})
		return r.query(ctx, query, p)
	}
	return "", nil
}

func (r *RAG) query(ctx context.Context, query, p string) (string, error) {
	content, err := r.QueryConfig.LLM.GenerateContent(ctx, []llm.Message{
		llm.NewSystemMessage("", p),
		llm.NewUserMessage("", query),
	})
	if err != nil {
		return "", err
	}
	return content.Content, nil
}

func (r *RAG) localQueryContext(ctx context.Context, query string) (string, error) {
	// 对 entity 进行召回，召回前20
	me := make(map[string]*rag.Entity)
	mr := make(map[string][]*rag.Relationship)
	encoding, _ := tiktoken.GetEncoding(rag.DefaultTokenEncoding)

	for _, e := range r.Entities {
		me[e.Title] = e
	}

	entities := r.getLevelEntities(_maxLevel)

	// 获取到 level <= _minLevel 的所有实体，召回相关的前20
	entities, err := r.QueryConfig.Retriever.Query(ctx, query, entities, 20)
	if err != nil {
		return "", err
	}

	// 进一步选择 relation
	for _, relation := range r.Relationships {
		mr[relation.Source.Title] = append(mr[relation.Source.Title], relation)
		mr[relation.Target.Title] = append(mr[relation.Target.Title], relation)
		mr[relation.Source.Title+_tupleDelimiter+relation.Target.Title] = append(
			mr[relation.Source.Title+_tupleDelimiter+relation.Target.Title], relation)
		mr[relation.Target.Title+_tupleDelimiter+relation.Source.Title] = append(
			mr[relation.Target.Title+_tupleDelimiter+relation.Source.Title], relation)
		mr[relation.Id] = append(mr[relation.Id], relation)
	}

	// 最后选择 text unit
	mt := make(map[string]*rag.TextUnit)
	for _, unit := range r.TextUnits {
		mt[unit.Id] = unit
	}

	mreport := make(map[int]*rag.Report)
	for _, report := range r.Reports {
		mreport[report.Community] = report
	}

	md := make(map[string]*rag.Document)
	for _, document := range r.Documents {
		md[document.Id] = document
	}

	// 开始拼接
	finalReports := queryRelatedCommunities(
		entities, me, mreport, encoding,
		r.QueryConfig.LLMMaxToken*_localCommunityPromptPercent/100)
	finalEntities, token := queryRelatedEntities(entities,
		me, encoding, r.QueryConfig.LLMMaxToken*_localEntityPromptPercent/100)
	finalTextUnits := queryRelatedTextUnits(entities,
		me, mt, encoding, r.QueryConfig.LLMMaxToken*_localTextUnitPromptPercent/100)
	finalDocuments := queryRelatedDocuments(finalTextUnits, md)
	finalRelations := make([]*rag.Relationship, 0, len(entities))
	added := make([]string, 0, len(entities))
	leftToken := r.QueryConfig.LLMMaxToken*_localEntityPromptPercent/100 - token
	for i := 0; i < len(entities) && leftToken > 0; i++ {
		added = append(added, entities[i])
		var count = 0
		finalRelations, count = queryRelatedRelationships(mr, added, encoding)
		leftToken -= count
	}
	// 拼接数据
	content := ""
	if len(finalReports) > 0 {
		content += "-----Reports-----\n"
		content += "title|content\n"
		for _, report := range finalReports {
			content += fmt.Sprintf("%s|%s\n", report.Title, report.Summary)
		}
	}

	if len(finalEntities) > 0 {
		content += "-----Entities-----\n"
		content += "entity|description|number of relationships\n"
		for _, entity := range finalEntities {
			content += fmt.Sprintf("%s|%s|%d\n",
				entity.Title, entity.Desc, entity.Degree)
		}
	}

	if len(finalRelations) > 0 {
		content += "-----Relations-----\n"
		content += "source|target|description|weight|links\n"
		for _, relation := range finalRelations {
			content += fmt.Sprintf("%s|%s|%s|%d|%d\n",
				relation.Source.Title, relation.Target.Title, relation.Desc,
				relation.Weight, relation.CombinedDegree)
		}
	}

	if len(finalTextUnits) > 0 {
		content += "-----Sources-----\n"
		content += "id|content\n"
		for i, unit := range finalTextUnits {
			content += fmt.Sprintf("%d|%s\n", i, unit.Text)
		}
	}

	if len(finalTextUnits) > 0 {
		content += "-----Source Document Link-----\n"
		content += "title|link\n"
		for _, doc := range finalDocuments {
			content += fmt.Sprintf("%s|%s\n", doc.Title,
				doc.Link)
		}
	}

	return content, nil
}

func (r *RAG) getLevelEntities(level int) []string {
	mc := make(map[int]*rag.Community)
	for _, c := range r.Communities {
		mc[c.Community] = c
	}
	entities := make([]string, 0, len(r.Entities))
	for _, entity := range r.Entities {
		for _, c := range entity.Communities {
			if mc[c].Level <= level {
				entities = append(entities, entity.Title)
				break
			}
		}
	}
	return entities
}

func queryRelatedRelationships(mr map[string][]*rag.Relationship,
	entities []string, encoding *tiktoken.Tiktoken) ([]*rag.Relationship, int) {
	inR := make([]*rag.Relationship, 0, 20)
	outR := make([]*rag.Relationship, 0, 20)
	for i := 0; i < len(entities); i++ {
		// 追加 out relation
		outR = append(outR, mr[entities[i]]...)
		if i == 0 {
			continue
		}
		for j := 0; j < i; j++ {
			if _, ok := mr[entities[i]+_tupleDelimiter+entities[j]]; ok {
				inR = append(inR, mr[entities[i]]...)
			}
		}
	}
	// 把 outR 中 去除 inR 部分
	inR = funk.Uniq(inR).([]*rag.Relationship)
	tmp := funk.Uniq(outR).([]*rag.Relationship)
	outR = make([]*rag.Relationship, 0, len(tmp))

	rExist := make(map[*rag.Relationship]struct{})

	for _, relation := range inR {
		rExist[relation] = struct{}{}
	}
	for _, relation := range tmp {
		if _, ok := rExist[relation]; ok {
			continue
		}
		outR = append(outR, relation)
	}

	sort.Slice(outR, func(i, j int) bool {
		return outR[i].Weight > outR[j].Weight
	})
	relations := append(inR, outR...)
	if len(relations) > _topKRelations*len(entities) {
		relations = relations[:_topKRelations*len(entities)]
	}

	token := 0
	for _, relation := range relations {
		token += len(encoding.Encode(relation.Source.Title, nil, nil)) +
			len(encoding.Encode(relation.Target.Title, nil, nil)) +
			len(encoding.Encode(relation.Desc, nil, nil))
	}

	return relations, token
}

func queryRelatedCommunities(entities []string,
	me map[string]*rag.Entity, mreport map[int]*rag.Report,
	encoding *tiktoken.Tiktoken, maxToken int) []*rag.Report {
	// 根据 entity 召回 相关的 community
	mcc := make(map[int]int)
	for _, e := range entities {
		for _, community := range me[e].Communities {
			mcc[community]++
		}
	}
	// 按照出现的频率进行排序
	communities := funk.Keys(mcc).([]int)
	relatedCc := funk.Values(mcc).([]int)
	sort.Slice(communities, func(i, j int) bool {
		return relatedCc[i] > relatedCc[j]
	})
	token := 0
	final := make([]*rag.Report, 0, len(relatedCc))
	for i := 0; i < len(communities); i++ {
		final = append(final, mreport[communities[i]])
		token += len(encoding.Encode(mreport[communities[i]].Summary,
			nil, nil)) +
			len(encoding.Encode(mreport[communities[i]].Title,
				nil, nil))
		if token > maxToken {
			break
		}
	}
	return final
}

func queryRelatedTextUnits(entities []string,
	me map[string]*rag.Entity, mt map[string]*rag.TextUnit,
	encoding *tiktoken.Tiktoken, maxToken int) []*rag.TextUnit {
	relatedText := make([]*rag.TextUnit, 0, len(entities))
	for _, e := range entities {
		for _, unitId := range me[e].TextUnitIds {
			relatedText = append(relatedText, mt[unitId])
		}
	}
	relatedText = funk.Uniq(relatedText).([]*rag.TextUnit)
	token := 0
	final := make([]*rag.TextUnit, 0, len(relatedText))
	for i := 0; i < len(relatedText); i++ {
		final = append(final, relatedText[i])
		if len(encoding.Encode(relatedText[i].Text,
			nil, nil))+token >= maxToken {
			break
		}
	}
	return final
}

func queryRelatedEntities(entities []string, me map[string]*rag.Entity,
	encoding *tiktoken.Tiktoken, maxToken int) ([]*rag.Entity, int) {
	final := make([]*rag.Entity, 0, len(entities))
	token := 0
	for i := 0; i < len(entities); i++ {
		final = append(final, me[entities[i]])
		token += len(encoding.Encode(entities[i], nil, nil)) +
			len(encoding.Encode(me[entities[i]].Desc, nil, nil)) +
			len(encoding.Encode(strconv.Itoa(me[entities[i]].Degree), nil, nil))
		if token > maxToken {
			break
		}
	}
	return final, token
}

func queryRelatedDocuments(textUnits []*rag.TextUnit,
	md map[string]*rag.Document) []*rag.Document {
	docs := make([]*rag.Document, 0, len(textUnits))
	for _, unit := range textUnits {
		for _, docId := range unit.DocumentIds {
			docs = append(docs, md[docId])
		}
	}
	return docs
}
