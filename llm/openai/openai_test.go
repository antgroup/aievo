package openai

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/tool/calculator"
)

func newTestClient(t *testing.T, opts ...Option) *LLM {
	t.Helper()

	client, err := New(
		WithToken(os.Getenv("OPENAI_API_KEY")),
		WithModel(os.Getenv("OPENAI_MODEL")),
		WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestMultiContentText(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	var content []llm.Message

	content = append(content,
		llm.NewSystemMessage("", "You are an assistant"),
		llm.NewUserMessage("", "使用计算器计算一下，3*4等于多少"))
	cal := &calculator.Calculator{}
	rsp, err := client.GenerateContent(context.Background(), content,
		llm.WithTools([]llm.Tool{
			{
				Type: "function",
				Function: &llm.FunctionDefinition{
					Name:        cal.Name(),
					Description: cal.Description(),
					Parameters:  cal.Schema(),
					Strict:      cal.Strict(),
				},
			},
		}))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(rsp)
	content = append(content, llm.NewAssistantMessage(
		"", "", rsp.ToolCalls))
	content = append(content, llm.NewToolMessage(
		rsp.ToolCalls[0].ID, "12"))
	rsp, err = client.GenerateContent(context.Background(), content,
		llm.WithTools([]llm.Tool{
			{
				Type: "function",
				Function: &llm.FunctionDefinition{
					Name:        cal.Name(),
					Description: cal.Description(),
					Parameters:  cal.Schema(),
					Strict:      cal.Strict(),
				},
			},
		}))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(rsp)
}
