package ollama

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/tool/calculator"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, opts ...Option) *LLM {
	t.Helper()
	var ollamaModel string
	if ollamaModel = os.Getenv("OLLAMA_TEST_MODEL"); ollamaModel == "" {
		t.Skip("OLLAMA_TEST_MODEL not set")
		return nil
	}

	opts = append([]Option{WithModel(ollamaModel)}, opts...)

	c, err := New(opts...)
	require.NoError(t, err)
	return c
}

func TestGenerateContent(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	ctx := context.Background()
	resp, err := client.Generate(ctx, "你是谁?")
	require.NoError(t, err)
	require.NotEmpty(t, resp)
	t.Logf("response: %s", resp.Content)
}

func TestMultiContentText(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	var content []llm.Message

	content = append(content,
		*llm.NewSystemMessage("", "You are an assistant"),
		*llm.NewUserMessage("", "使用计算器计算一下，3*4等于多少"))
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
}
