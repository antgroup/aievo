package openai

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/tool/calculator"
	"github.com/antgroup/aievo/utils/json"
	goopenai "github.com/sashabaranov/go-openai"
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

func TestMultiContent(t *testing.T) {
	t.Parallel()

	client := newTestClient(t)

	var content []llm.Message

	content = append(content,
		*llm.NewUserMessage("", "hello"))
	rsp, err := client.GenerateContent(context.Background(), content,
		llm.WithLogProbes(true),
		llm.WithTopLogProbs(5))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(rsp)
}

func TestJson(t *testing.T) {
	a := `{"id":"da8d5dc7-3067-496a-963d-14e71f57c878","object":"chat.completion.chunk","created":1741081154,"model":"deepseek-chat","system_fingerprint":"fp_3a5770e1b4_prod0225","choices":[{"index":0,"delta":{"content":"Hello"},"logprobs":{"content":[{"token":"Hello","logprob":0.0,"bytes":[72,101,108,108,111],"top_logprobs":[{"token":"Hello","logprob":0.0,"bytes":[72,101,108,108,111]},{"token":"Hi","logprob":-21.107803,"bytes":[72,105]},{"token":"Hey","logprob":-40.61815,"bytes":[72,101,121]},{"token":"How","logprob":-43.581818,"bytes":[72,111,119]},{"token":"###","logprob":-43.834167,"bytes":[35,35,35]}]}]},"finish_reason":null}]}`
	resp := goopenai.ChatCompletionStreamResponse{}
	err := json.Unmarshal([]byte(a), &resp)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp)
}

func TestEnv(t *testing.T) {
	fmt.Println(os.Environ())
}
