package rag

import (
	"context"

	"github.com/antgroup/aievo/llm"
	"gorm.io/gorm"
)

const (
	DefaultTokenEncoding = "cl100k_base"
)

type WorkflowConfig struct {
	ChunkSize          int
	ChunkOverlap       int
	Separators         []string
	MaxToken           int
	EntityTypes        []string
	LLM                llm.LLM
	LLMCallConcurrency int
	DB                 *gorm.DB
}

type QueryConfig struct {
	LLM         llm.LLM
	LLMMaxToken int
	Retriever   Retriever
	MaxTurn     int
}

type WorkflowContext struct {
	Id       int64
	BasePath string
	// config for index
	Config *WorkflowConfig
	// config for query
	QueryConfig      *QueryConfig
	Documents        []*Document
	TextUnits        []*TextUnit
	Relationships    []*Relationship
	TmpRelationships []*TmpRelationship
	Entities         []*Entity
	Communities      []*Community
	Nodes            []*Node
	Reports          []*Report
}

func NewWorkflowContext() *WorkflowContext {
	ctx := &WorkflowContext{}
	ctx.Documents = make([]*Document, 0)
	ctx.TextUnits = make([]*TextUnit, 0)
	ctx.Relationships = make([]*Relationship, 0)
	ctx.TmpRelationships = make([]*TmpRelationship, 0)
	ctx.Entities = make([]*Entity, 0)
	ctx.Communities = make([]*Community, 0)
	ctx.Nodes = make([]*Node, 0)
	ctx.Reports = make([]*Report, 0)
	return ctx
}

type Progress func(ctx context.Context, args *WorkflowContext) error

type Document struct {
	Id          string   `json:"id"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	TextUnitIds []string `json:"text_unit_ids"`
	Link        string   `json:"link"`
}

type TextUnit struct {
	Id              string   `json:"id"`
	Text            string   `json:"text"`
	DocumentIds     []string `json:"document_ids"`
	EntityIds       []string `json:"entity_ids"`
	RelationshipIds []string `json:"relationship_ids"`
	NumToken        int      `json:"num_token"`
}

type Entity struct {
	Id          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Desc        string   `json:"description"`
	Degree      int      `json:"degree"`
	TmpDesc     []string `json:"-"`
	Communities []int    `json:"communities"`
	TextUnitIds []string `json:"text_unit_ids"`
}

type Relationship struct {
	Id             string   `json:"id"`
	Source         *Entity  `json:"source"`
	Target         *Entity  `json:"target"`
	Desc           string   `json:"desc"`
	Weight         float64  `json:"weight"`
	CombinedDegree int      `json:"combined_degree"`
	TextUnitIds    []string `json:"text_unit_ids"`
}

type TmpRelationship struct {
	Id             string   `json:"id"`
	Source         string   `json:"source"`
	Target         string   `json:"target"`
	Desc           string   `json:"desc"`
	TmpDesc        []string `json:"-"`
	Weight         float64  `json:"weight"`
	CombinedDegree int      `json:"combined_degree"`
	TextUnitIds    []string `json:"text_unit_ids"`
	SourceId       string   `json:"source_id"`
}

type Node struct {
	Id        string `json:"id"`
	Title     string `json:"title"`
	Community int    `json:"community"`
	Level     int    `json:"level"`
	Degree    int    `json:"degree"`
}

type Community struct {
	Id              string   `json:"id"`
	Title           string   `json:"title"`
	Community       int      `json:"community"`
	Level           int      `json:"level"`
	RelationshipIds []string `json:"relationship_ids"`
	TextUnitIds     []string `json:"text_unit_ids"`
	Parent          int      `json:"parent"`
	EntityIds       []string `json:"entity_ids"`
	Period          string   `json:"period"`
	Size            int      `json:"size"`
}

type Report struct {
	Community         int        `json:"community"`
	Title             string     `json:"title"`
	Summary           string     `json:"summary"`
	Rating            float64    `json:"rating"`
	RatingExplanation string     `json:"rating_explanation"`
	Findings          []*Finding `json:"findings"`
}
type Finding struct {
	Summary     string `json:"summary"`
	Explanation string `json:"explanation"`
}

type HierarchicalCluster struct {
	Node           string `json:"node"`
	Cluster        int    `json:"cluster"`
	ParentCluster  int    `json:"parent_cluster"`
	Level          int    `json:"level"`
	IsFinalCluster bool   `json:"is_final_cluster"`
}
