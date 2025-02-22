package index

import (
	"context"
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag/index/prompts"
	"github.com/antgroup/aievo/utils/parallel"
)

var (
	_completionDelimiter = "<|COMPLETE|>"
	_tupleDelimiter      = "|"
	_recordDelimiter     = "##"
)

func ExtraGraph(ctx context.Context, args *WorkflowContext) error {
	return nil
}

func extractEntities(ctx context.Context, args *WorkflowContext) error {
	template, _ := prompt.NewPromptTemplate(prompts.ExtraGraph)

	parallel.Parallel(func(i int) any {
		p, _ := template.Format(map[string]any{
			"entity_types":         strings.Join(args.config.EntityTypes, ","),
			"tuple_delimiter":      _tupleDelimiter,
			"record_delimiter":     _recordDelimiter,
			"completion_delimiter": _completionDelimiter,
			"input_text":           args.TextUnits[i].Text,
		})
		args.config.LLM.GenerateContent(ctx,
			[]llm.Message{llm.NewUserMessage("", p)},
			llm.WithTemperature(0.1))

		return nil
	}, len(args.TextUnits), args.config.LLMCallConcurrency)
	return nil
}

func summarizeDescriptions(ctx context.Context, args *WorkflowContext) error {
	return nil
}
