package query

import (
	"github.com/antgroup/aievo/rag"
)

const (
	_maxLevel                    = 2
	_localEntityPromptPercent    = 35
	_localCommunityPromptPercent = 15
	_localTextUnitPromptPercent  = 50

	_tupleDelimiter = "<|>"
	_topKRelations  = 10
)

type Method string

const (
	Local  Method = "local"
	Global Method = "global"
	COT    Method = "cot"
)

type RAG struct {
	*rag.WorkflowContext
}

type Response struct {
	Query   string `json:"retrieval"`
	Reason  string `json:"reason"`
	Summary string `json:"summary"`
	Answer  string `json:"answer"`
}
