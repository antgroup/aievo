package qwen

import (
	"context"
	"fmt"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/tool/calculator"
	"log"
	"testing"
)

func newTestClient(t *testing.T, opts ...Option) *LLM {
	t.Helper()

	//client, err := New(
	//	WithToken(os.Getenv("QWEN_API_KEY")),
	//	WithModel(os.Getenv("QWEN_MODEL")),
	//	WithBaseURL(os.Getenv("QWEN_BASE_URL")),
	//	WithStream(true))
	//if err != nil {
	//	t.Fatal(err)
	//}
	client, err := New(
		WithToken("sk-13e4a662584a418690dc65e80c4fd035"),
		WithModel("qwen-plus"),
		WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
		WithStream(true))
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func TestLLM_GenerateContent(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	var content []llm.Message

	content = append(content,
		*llm.NewSystemMessage("", "You are an assistant "),
		*llm.NewUserMessage("", "使用计算器计算一下，50*300等于多少"))
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
	content = append(content, *llm.NewAssistantMessage(
		"", "", rsp.ToolCalls))
	content = append(content, *llm.NewToolMessage(
		rsp.ToolCalls[0].ID, "15000"))
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