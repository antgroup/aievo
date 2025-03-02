package index

import (
	"context"

	"github.com/antgroup/aievo/rag"
	"github.com/thoas/go-funk"
)

func FinalDocuments(_ context.Context, args *rag.WorkflowContext) error {
	m := make(map[string]*rag.Document)
	for _, document := range args.Documents {
		m[document.Id] = document
	}
	for _, unit := range args.TextUnits {
		for _, documentId := range unit.DocumentIds {
			m[documentId].TextUnitIds = append(
				m[documentId].TextUnitIds, unit.Id)
		}
	}
	for _, document := range args.Documents {
		document.TextUnitIds = funk.UniqString(document.TextUnitIds)
	}

	return nil
}
