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

type RAG struct {
	Context *rag.WorkflowContext
}
