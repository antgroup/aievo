package query

import (
	"context"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/rag/prompts"
	"github.com/antgroup/aievo/utils/json"
)

func (r *RAG) Query(ctx context.Context, query string) (string, error) {
	ret, err := r.localQueryContext(ctx, query)
	if err != nil {
		return "", err
	}
	t, err := prompt.NewPromptTemplate(prompts.COTPrompt)
	if err != nil {
		return "", err
	}

	content := query + ret
	// 组装content
	resp := &Response{}
	for i := 0; i < r.QueryConfig.MaxTurn; i++ {
		p, err := t.Format(map[string]any{
			"task":    query,
			"context": content,
		})
		if err != nil {
			return "", err
		}
		output, err := r.QueryConfig.LLM.GenerateContent(ctx,
			[]llm.Message{llm.NewUserMessage("", p)})
		if err != nil {
			return "", err
		}
		resp = &Response{}
		err = json.Unmarshal([]byte(output.Content), resp)
		if err != nil {
			return "", err
		}
		if resp.Answer != "" {
			return resp.Answer, nil
		}
		// 组装上下文
		content = output.Content
	}
	return resp.Summary, nil
}
