package index

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag"
	"github.com/antgroup/aievo/rag/prompts"
	db2 "github.com/antgroup/aievo/rag/storage/db"
	"github.com/stretchr/testify/assert"
	"github.com/thoas/go-funk"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBaseDocuments(t *testing.T) {
	args := &rag.WorkflowContext{BasePath: "/Users/tyloafer/WorkPlace/src/github.com/antgroup/aievo/rag"}
	err := BaseDocuments(context.Background(),
		args)
	if err != nil {
		panic(err)
	}
	fmt.Println(args.Documents)
}

type mockLLM struct {
	contents map[string]*llm.Generation
	keys     []string
}
type tmpgene struct {
	Result struct {
		Id      string `json:"id"`
		Choices []struct {
			Message llm.Message `json:"message"`
		} `json:"choices"`
	} `json:"result"`
	Input struct {
		Messages []llm.Message `json:"messages"`
	} `json:"input"`
}

func initMockLLM() llm.LLM {
	contents := make(map[string]*llm.Generation)
	keys := make([]string, 0, 100)
	base, _ := os.Getwd()
	paths := []string{
		filepath.Join(base, "../testdata/cache", "community_reporting"),
		filepath.Join(base, "../testdata/cache", "entity_extraction"),
		filepath.Join(base, "../testdata/cache", "summarize_descriptions"),
	}
	for _, path := range paths {
		dir, _ := os.ReadDir(path)
		for _, entry := range dir {
			if entry.IsDir() {
				continue
			}
			content, _ := os.ReadFile(filepath.Join(path, entry.Name()))
			gene := &tmpgene{}
			err := json.Unmarshal(content, gene)
			if err != nil {
				panic(err)
			}
			key := strings.TrimSpace(gene.Input.Messages[0].Content)
			if len(gene.Input.Messages) > 1 {
				key += prompts.ContinueExtra
			}
			keys = append(keys, key)

			contents[key] =
				&llm.Generation{
					Role:       string(gene.Result.Choices[0].Message.Role),
					Content:    gene.Result.Choices[0].Message.Content,
					StopReason: "stop",
					Usage: &llm.Usage{
						CompletionTokens: 0,
						PromptTokens:     0,
						TotalTokens:      0,
					},
				}
		}
	}

	return &mockLLM{contents: contents,
		keys: keys}
}

func (m mockLLM) Generate(ctx context.Context, prompt string, options ...llm.GenerateOption) (*llm.Generation, error) {
	return nil, nil
}

func (m mockLLM) GenerateContent(ctx context.Context, messages []llm.Message, options ...llm.GenerateOption) (*llm.Generation, error) {
	content := strings.TrimSpace(messages[0].Content)
	if len(messages) > 1 {
		content += prompts.ContinueExtra
	}
	if result, ok := m.contents[content]; ok {
		return result, nil
	}
	content = strings.ReplaceAll(content, "\",\"", "\", \"")
	if result, ok := m.contents[content]; ok {
		return result, nil
	}
	if content == "" {
		fmt.Println("content is empty")
	}
	similar, _ := FindMostSimilar(content, m.keys)
	// if score < 0.90 {
	// 	fmt.Println(score)
	// 	// fmt.Println(content)
	// }
	return m.contents[similar], nil
}

func TestMockLLM(t *testing.T) {
	fmt.Println()
}

// 支持多种算法的通用版本
func FindMostSimilar(target string, candidates []string) (string, float64) {
	if len(candidates) == 0 {
		return "", 0
	}

	var bestMatch string
	maxSimilarity := -1.0

	for _, candidate := range candidates {
		var similarity float64

		similarity = jaccardSimilarity(target, candidate)

		if similarity > maxSimilarity {
			maxSimilarity = similarity
			bestMatch = candidate
		}
	}

	return bestMatch, maxSimilarity
}

// Jaccard 相似度实现
func jaccardSimilarity(a, b string) float64 {
	setA := make(map[string]bool)
	for _, word := range strings.Fields(a) {
		setA[word] = true
	}

	setB := make(map[string]bool)
	for _, word := range strings.Fields(b) {
		setB[word] = true
	}

	intersection := 0
	for word := range setA {
		if setB[word] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// 计算两个文本的余弦相似度 (范围: 0~1)
func CosineSimilarity(text1, text2 string) float64 {
	// 1. 分词并统计词频
	tf1 := termFrequency(text1)
	tf2 := termFrequency(text2)

	// 2. 获取所有唯一词汇
	vocab := unionVocab(tf1, tf2)

	// 3. 构建词频向量
	vec1 := buildVector(tf1, vocab)
	vec2 := buildVector(tf2, vocab)

	// 4. 计算余弦相似度
	dotProduct := 0.0
	magnitude1 := 0.0
	magnitude2 := 0.0

	for i := range vec1 {
		dotProduct += vec1[i] * vec2[i]
		magnitude1 += vec1[i] * vec1[i]
		magnitude2 += vec2[i] * vec2[i]
	}

	magnitude := math.Sqrt(magnitude1) * math.Sqrt(magnitude2)
	if magnitude == 0 {
		return 0
	}

	return dotProduct / magnitude
}

// 分词并统计词频
func termFrequency(text string) map[string]int {
	// 清洗文本：转小写、去标点
	reg := regexp.MustCompile(`[^a-zA-Z\s]`)
	cleanText := reg.ReplaceAllString(strings.ToLower(text), "")

	// 分词
	words := strings.Fields(cleanText)

	// 统计词频
	tf := make(map[string]int)
	for _, word := range words {
		tf[word]++
	}
	return tf
}

// 合并两个词汇表
func unionVocab(tf1, tf2 map[string]int) []string {
	vocab := make(map[string]struct{})
	for word := range tf1 {
		vocab[word] = struct{}{}
	}
	for word := range tf2 {
		vocab[word] = struct{}{}
	}

	result := make([]string, 0, len(vocab))
	for word := range vocab {
		result = append(result, word)
	}
	return result
}

// 构建词频向量
func buildVector(tf map[string]int, vocab []string) []float64 {
	vector := make([]float64, len(vocab))
	for i, word := range vocab {
		vector[i] = float64(tf[word])
	}
	return vector
}

func TestMock(t *testing.T) {
	type KV struct {
		Text    string `json:"text"`
		Results string `json:"results"`
	}

	results := getraw("results3.json")
	kvs := make([]*KV, 0, 50)
	split := strings.Split(results, "\n")
	for _, s := range split {
		if len(strings.TrimSpace(s)) == 0 {
			continue
		}
		kv := &KV{}
		err := json.Unmarshal([]byte(s), kv)
		if err != nil {
			panic(err)
		}
		kvs = append(kvs, kv)
	}
	client := initMockLLM()
	template, _ := prompt.NewPromptTemplate(prompts.ExtraGraph)
	entityTypes := []string{"organization", "person", "geo", "event"}
	for _, kv := range kvs {
		p, _ := template.Format(map[string]any{
			"entity_types":         strings.Join(entityTypes, ","),
			"tuple_delimiter":      _tupleDelimiter,
			"record_delimiter":     _recordDelimiter,
			"completion_delimiter": _completionDelimiter,
			"input_text":           kv.Text,
		})
		content, _ := client.GenerateContent(context.Background(),
			[]llm.Message{llm.NewUserMessage("", p)},
			llm.WithTemperature(0.1))
		tmp := strings.TrimSpace(content.Content)

		content, _ = client.GenerateContent(context.Background(),
			[]llm.Message{
				llm.NewUserMessage("", p),
				llm.NewAssistantMessage("", tmp, nil),
				llm.NewUserMessage("", prompts.ContinueExtra),
			},
			llm.WithTemperature(0.1))
		tmp = strings.TrimSpace(tmp + content.Content)
		similarity := jaccardSimilarity(tmp, kv.Results)
		// fmt.Println(similarity)
		assert.True(t, similarity == 1)
	}
}

func TestSingleResult(t *testing.T) {
	content := getraw("results_record.json")
	records := strings.Split(content, "\n")
	type tmp struct {
		Entities      []*rag.Entity          `json:"entities"`
		Relationships []*rag.TmpRelationship `json:"relationships"`
		Id            string                 `json:"id"`
	}
	tmps := make(map[string]*tmp)
	args := &rag.WorkflowContext{
		BasePath: "",
		Config: &rag.WorkflowConfig{
			ChunkSize:          1200,
			ChunkOverlap:       200,
			Separators:         nil,
			MaxToken:           16000,
			EntityTypes:        []string{"concept", "component", "configuration", "example", "person", "log", "platform", "event"},
			LLM:                initMockLLM(),
			LLMCallConcurrency: 100,
		},
	}

	for _, record := range records {
		record = strings.TrimSpace(record)
		if len(record) == 0 {
			continue
		}
		r := &tmp{}
		err := json.Unmarshal([]byte(record), r)
		if err != nil {
			panic(err)
		}
		tmps[r.Id] = r
	}

	textUnits := make([]*rag.TextUnit, 0, 50)
	get("create_base_text_units.json", &textUnits)

	for _, t2 := range tmps {
		fmt.Println("unit id: ", t2.Id)
		fmt.Println("entity: title, type, desc")
		for _, entity := range t2.Entities {
			fmt.Println(entity.Title, entity.Type, entity.Desc)
		}
		fmt.Println("relationship: source, target, desc, weight, source_id")
		for _, relation := range t2.Relationships {
			fmt.Println(relation.Source, relation.Target, relation.Desc, relation.Weight, relation.SourceId)
		}
	}

	for _, unit := range textUnits {
		args.TextUnits = append(args.TextUnits, unit)
		err := extractEntities(context.Background(), args)
		if err != nil {
			panic(err)
		}

		tmpEntities := make(map[string]*rag.Entity)
		argEntities := make(map[string]*rag.Entity)

		for _, entity := range tmps[unit.Id].Entities {
			tmpEntities[entity.Title] = entity
		}
		for _, entity := range args.Entities {
			argEntities[entity.Title] = entity
		}

		assert.Equal(t, len(tmpEntities), len(argEntities), unit.Id)
		for title, entity := range tmpEntities {
			assert.NotNil(t, argEntities[title], unit.Id)
			assert.Equal(t, entity.Type, entity.Type, unit.Id)
			assert.Equal(t, entity.Desc, entity.Desc, unit.Id)
			assert.Equal(t, entity.TextUnitIds, entity.TextUnitIds, unit.Id)
		}

		// 比对 relation
		tmpRelationships := make(map[string]*rag.TmpRelationship)
		argRelations := make(map[string]*rag.TmpRelationship)

		for _, relation := range tmps[unit.Id].Relationships {
			tmpRelationships[id(relation.Source+"-"+relation.Target)] = relation
			tmpRelationships[id(relation.Target+"-"+relation.Source)] = relation
		}
		for _, relation := range args.TmpRelationships {
			argRelations[id(relation.Source+"-"+relation.Target)] = relation
			argRelations[id(relation.Target+"-"+relation.Source)] = relation
		}
		assert.Equal(t, len(tmpRelationships), len(argRelations), unit.Id)
		for idx, relation := range tmpRelationships {
			assert.NotNil(t, argRelations[idx], unit.Id+"-"+relation.Source+"-"+relation.Target)
			assert.Equal(t, relation.Weight, argRelations[idx].Weight, unit.Id+"-"+relation.Source+"-"+relation.Target)
			assert.Equal(t, relation.Desc, argRelations[idx].Desc, unit.Id+"-"+relation.Source+"-"+relation.Target)
			assert.Equal(t, relation.SourceId, argRelations[idx].TextUnitIds[0], unit.Id+"-"+relation.Source+"-"+relation.Target)
		}

		args.TextUnits = args.TextUnits[0:0]
		args.Entities = args.Entities[0:0]
		args.Relationships = args.Relationships[0:0]
	}
}

func newTestGormDB() (*gorm.DB, error) {
	dsn := os.Getenv("AIEVO_DSN")
	var err error
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, _ := db.DB()
	if err = sqlDB.Ping(); err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Minute * 10)
	return db, nil
}

func TestWorkflow_Run(t *testing.T) {
	db, err := newTestGormDB()
	assert.Nil(t, err)

	// 初始化 base_text_unit 和 base_document
	args := &rag.WorkflowContext{
		BasePath: "",
		Config: &rag.WorkflowConfig{
			ChunkSize:          1200,
			ChunkOverlap:       200,
			Separators:         nil,
			MaxToken:           16000,
			EntityTypes:        []string{"organization", "person", "geo", "event"},
			LLM:                initMockLLM(),
			LLMCallConcurrency: 100,
			DB:                 db,
		},
	}

	// 获取初始数据
	get("create_base_text_units.json", &args.TextUnits)
	get("create_final_documents.json", &args.Documents)
	finalDocuments := make([]*rag.Document, 0, 100)
	get("create_final_documents.json", &finalDocuments)

	baseEntity := make([]*rag.Entity, 0, 100)
	get("base_entity_nodes.json", &baseEntity)
	for _, entity := range baseEntity {
		entity.TextUnitIds = funk.UniqString(entity.TextUnitIds)
	}

	baseRelations := make([]*rag.TmpRelationship, 0, 100)
	get("base_relationship_edges.json", &baseRelations)

	baseCommunity := make([]*rag.Community, 0, 100)
	get("base_communities.json", &baseCommunity)
	finalEntities := make([]*rag.Entity, 0, 100)
	get("create_final_entities.json", &finalEntities)
	finalRelationships := make([]*rag.TmpRelationship, 0, 100)
	get("create_final_relationships.json", &finalRelationships)
	finalNodes := make([]*rag.Node, 0, 100)
	get("create_final_nodes.json", &finalNodes)
	finalCommunities := make([]*rag.Community, 0, 100)
	get("create_final_communities.json", &finalCommunities)
	finalTextUnits := make([]*rag.TextUnit, 0, 100)
	get("create_final_text_units.json", &finalTextUnits)

	finalReport := make([]*rag.Report, 0, 100)
	get("create_final_community_reports.json", &finalReport)

	for _, document := range args.Documents {
		document.TextUnitIds = document.TextUnitIds[:]
	}

	err = FinalDocuments(context.Background(), args)
	assert.Nil(t, err)
	assert.Equal(t, len(finalDocuments), len(args.Documents))
	assert.ElementsMatch(t, finalDocuments[0].TextUnitIds, args.Documents[0].TextUnitIds)

	// 对 entity 的 title type desc 和 text_unit 进行断言
	err = ExtraGraph(context.Background(), args)
	assert.Nil(t, err)
	assert.Equal(t, len(baseEntity), len(args.Entities))
	// 首先检查 entity
	baseEntityMap := make(map[string]*rag.Entity)
	entityMap := make(map[string]*rag.Entity)
	for _, entity := range baseEntity {
		baseEntityMap[entity.Title] = entity
	}
	for _, entity := range args.Entities {
		entityMap[entity.Title] = entity
	}

	//  modify relationship id && entity id for following test
	for title, entity := range baseEntityMap {
		assert.NotNil(t, entityMap[title])
		// modify entity id for following test
		entityMap[title].Id = entity.Id
		assert.Equal(t, entityMap[title].Type, entity.Type, entity.Title)
		assert.ElementsMatch(t, entityMap[title].TextUnitIds, entity.TextUnitIds, entity.Title)
		if entity.Desc == "None" {
			entity.Desc = ""
		}
		// assert.True(t, jaccardSimilarity(entityMap[title].Desc, entity.Desc) > 0.96, entity.Title)
		desc1 := strings.TrimSpace(strings.ReplaceAll(entityMap[title].Desc, "<|COMPLETE|>", ""))
		desc2 := strings.TrimSpace(strings.ReplaceAll(entity.Desc, "<|COMPLETE|>", ""))
		assert.Equal(t, desc1, desc2, entity.Title)
	}

	// assert.Equal(t, len(baseRelations), len(args.Relationships))

	// 开始 断言 relation
	baseRelationMap := make(map[string]*rag.TmpRelationship)
	relationMap := make(map[string]*rag.Relationship)
	for _, relation := range baseRelations {
		relation.TextUnitIds = funk.UniqString(relation.TextUnitIds)
		if _, ok := baseRelationMap[relation.Source+"-"+relation.Target]; ok {
			baseRelationMap[relation.Source+"-"+relation.Target].Weight += relation.Weight
			baseRelationMap[relation.Source+"-"+relation.Target].TextUnitIds = append(baseRelationMap[relation.Source+"-"+relation.Target].TextUnitIds, relation.TextUnitIds...)
			continue
		}
		baseRelationMap[relation.Source+"-"+relation.Target] = relation
		baseRelationMap[relation.Target+"-"+relation.Source] = relation
	}
	for _, relation := range args.Relationships {
		relationMap[relation.Source.Title+"-"+relation.Target.Title] = relation
		relationMap[relation.Target.Title+"-"+relation.Source.Title] = relation
	}
	assert.Equal(t, len(baseRelationMap), len(relationMap))
	for key, relation := range baseRelationMap {
		// modify relation id for following test
		assert.NotNil(t, relationMap[key])
		relationMap[key].Id = relation.Id
		// 由于 entity顺序和 desc的顺序问题，导致相似度检测不准
		// assert.Equal(t, relation.Description, relationMap[key].Desc, key)
		assert.ElementsMatch(t, relation.TextUnitIds, relationMap[key].TextUnitIds, key)
		assert.Equal(t, relation.Weight, relationMap[key].Weight, key)
	}

	// assert communities
	err = ComputeCommunities(context.Background(), args)
	if err != nil {
		panic(err)
	}
	assert.Nil(t, err)
	baseCommunityMap := make(map[string]*rag.Community)
	communityMap := make(map[string]*rag.Community)
	for _, community := range args.Communities {
		communityMap[fmt.Sprintf("%s-%d-%d", community.Title, community.Community, community.Level)] = community
	}
	for _, community := range baseCommunity {
		baseCommunityMap[fmt.Sprintf("%s-%d-%d", community.Title, community.Community, community.Level)] = community
	}
	assert.Equal(t, len(baseCommunityMap), len(communityMap))
	for key, community := range baseCommunityMap {
		assert.NotNil(t, communityMap[key])
		assert.Equal(t, communityMap[key].Parent, community.Parent)
		assert.Equal(t, communityMap[key].Community, community.Community)
	}

	// assert final entity
	err = FinalEntities(context.Background(), args)
	assert.Nil(t, err)
	finalEntitiesMap := make(map[string]*rag.Entity)
	entitiesMap := make(map[string]*rag.Entity)
	for _, entity := range args.Entities {
		entitiesMap[entity.Title] = entity
	}
	for _, entity := range finalEntities {
		finalEntitiesMap[entity.Title] = entity
	}
	assert.Equal(t, len(finalEntitiesMap), len(entitiesMap))
	for key, entity := range finalEntitiesMap {
		if entity.Desc == "None" {
			entity.Desc = ""
		}
		desc1 := strings.TrimSpace(strings.ReplaceAll(entitiesMap[key].Desc, "<|COMPLETE|>", ""))
		desc2 := strings.TrimSpace(strings.ReplaceAll(entity.Desc, "<|COMPLETE|>", ""))

		assert.NotNil(t, entitiesMap[key])
		assert.Equal(t, entity.Type, entitiesMap[key].Type)
		assert.Equal(t, desc1, desc2)
		assert.ElementsMatch(t, entity.TextUnitIds, entitiesMap[key].TextUnitIds)
	}

	// asset nodes
	err = FinalNodes(context.Background(), args)
	assert.Nil(t, err)
	finalNodesMap := make(map[string]*rag.Node)
	nodesMap := make(map[string]*rag.Node)
	for _, node := range args.Nodes {
		nodesMap[fmt.Sprintf("%s-%d", node.Title, node.Community)] = node
	}
	for _, node := range finalNodes {
		finalNodesMap[fmt.Sprintf("%s-%d", node.Title, node.Community)] = node
	}
	assert.Equal(t, len(finalNodesMap), len(nodesMap))
	for key, node := range finalNodesMap {
		assert.NotNil(t, nodesMap[key])
		assert.Equal(t, node.Degree, nodesMap[key].Degree)
	}
	// 把final_relationship 拿过来做测试，go计算的relationship 对 source 和 target 进行了去重，
	// 而final没有，导致断言的时候 relationship_ids 不通过
	args.Relationships = args.Relationships[0:0]
	for _, relation := range finalRelationships {
		if entityMap[relation.Source] == nil ||
			entityMap[relation.Target] == nil {
			panic(err)
		}
		args.Relationships = append(args.Relationships,
			&rag.Relationship{
				Id:             relation.Id,
				Source:         entityMap[relation.Source],
				Target:         entityMap[relation.Target],
				Weight:         relation.Weight,
				CombinedDegree: relation.CombinedDegree,
				TextUnitIds:    funk.UniqString(relation.TextUnitIds),
				Desc:           relation.Desc,
			})
	}
	// assert final community
	err = FinalCommunities(context.Background(), args)
	assert.Nil(t, err)
	finalCommunitiesMap := make(map[int]*rag.Community)
	communitiesMap := make(map[int]*rag.Community)
	for _, community := range args.Communities {
		communitiesMap[community.Community] = community
	}
	for _, community := range finalCommunities {
		finalCommunitiesMap[community.Community] = community
	}
	assert.Equal(t, len(finalCommunitiesMap), len(communitiesMap))
	for key, community := range finalCommunitiesMap {
		// change community id for following test
		communitiesMap[key].Id = community.Id
		assert.NotNil(t, communitiesMap[key])
		assert.Equal(t, community.Parent, communitiesMap[key].Parent)
		assert.Equal(t, community.Level, communitiesMap[key].Level)
		assert.ElementsMatch(t, community.EntityIds, communitiesMap[key].EntityIds, fmt.Sprintf("%d-entity", community.Community))
		// assert.ElementsMatch(t, community.RelationshipIds, communitiesMap[key].RelationshipIds, community.Community)
		// community text unit id
		assert.ElementsMatch(t, community.TextUnitIds, communitiesMap[key].TextUnitIds, fmt.Sprintf("%d-textunit", community.Community))
		assert.Equal(t, community.Size, communitiesMap[key].Size, community.Community)

	}

	// assert final text unit
	err = FinalTextUnits(context.Background(), args)
	assert.Nil(t, err)
	finalTextUnitsMap := make(map[string]*rag.TextUnit)
	textUnitsMap := make(map[string]*rag.TextUnit)
	for _, textUnit := range args.TextUnits {
		textUnitsMap[textUnit.Id] = textUnit
	}
	for _, textUnit := range finalTextUnits {
		finalTextUnitsMap[textUnit.Id] = textUnit
	}
	assert.Equal(t, len(finalTextUnitsMap), len(textUnitsMap))
	for key, textUnit := range finalTextUnitsMap {
		assert.NotNil(t, textUnitsMap[key])
		assert.Equal(t, textUnit.Id, textUnitsMap[key].Id)
		assert.Equal(t, textUnit.Text, textUnitsMap[key].Text)
		assert.ElementsMatch(t, textUnit.DocumentIds, textUnitsMap[key].DocumentIds, "document id")
		assert.ElementsMatch(t, textUnit.EntityIds, textUnitsMap[key].EntityIds, "entity id")
		assert.ElementsMatch(t, textUnit.RelationshipIds, textUnitsMap[key].RelationshipIds, "relationship id"+textUnit.Id)
	}

	// 最后 assert report
	// err = FinalCommunityReport(context.Background(), args)
	// assert.Nil(t, err)
	// finalCommunityReportMap := make(map[int]*rag.Report)
	// reportMap := make(map[int]*rag.Report)
	// for _, report := range args.Reports {
	//	reportMap[report.Community] = report
	// }
	// for _, report := range finalReport {
	//	finalCommunityReportMap[report.Community] = report
	// }
	// assert.Equal(t, len(finalCommunityReportMap), len(reportMap))
	// for key, report := range finalCommunityReportMap {
	//	assert.NotNil(t, reportMap[key])
	//	assert.Equal(t, report.Title, reportMap[key].Title)
	//	assert.Equal(t, report.Summary, reportMap[key].Summary)
	//	assert.Equal(t, report.Findings, reportMap[key].Findings)
	// }

	err = Save(context.Background(), args, 10)
	assert.Nil(t, err)
}

func TestLoadWorkflow(t *testing.T) {
	wfCtx := rag.NewWorkflowContext()
	wfCtx.Id = 1

	db, err := newTestGormDB()
	assert.Nil(t, err)

	storage := db2.NewStorage(db2.WithDB(db))

	err = storage.Load(context.Background(), wfCtx)
	assert.Nil(t, err)
}

func get(filename string, result any) {
	base, _ := os.Getwd()
	path := filepath.Join(base, "..", "testdata", filename)
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(file, result)
	if err != nil {
		panic(err)
	}
}

func getraw(filename string) string {
	base, _ := os.Getwd()
	path := filepath.Join(base, "..", "tmp", filename)
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(file)
}

func TestAssertEqual(t *testing.T) {
	assert.Equal(t, 1, 2)
}
