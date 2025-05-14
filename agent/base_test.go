package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/antgroup/aievo/feedback"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/tool/calculator"
	"github.com/antgroup/aievo/tool/mcp"
	"github.com/goccy/go-graphviz"
)

func client() llm.LLM {
	c, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func TestBaseAgent(t *testing.T) {
	base, err := NewBaseAgent(
		WithLLM(client()),
		WithName("test"),
		WithDesc("test"),
		WithTools([]tool.Tool{
			calculator.Calculator{},
		}),
		WithFeedbacks(&feedback.ContentFeedback{}))
	if err != nil {
		log.Fatal(err)
	}
	run, err := base.Run(context.Background(), []schema.Message{
		{
			Sender:   "User",
			Receiver: base.name,
			Content:  "20乘以30等于几",
			Type:     "Msg",
		},
	},
		llm.WithTemperature(0.1),
		llm.WithTopP(0.8),
		llm.WithRepetitionPenalty(1.05),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", run)
}

func TestConversationAgent(t *testing.T) {
	base, err := NewBaseAgent(
		WithLLM(client()),
		WithName("test"),
		WithDesc("test"))
	if err != nil {
		panic(err)
	}
	run, err := base.Run(context.Background(),
		[]schema.Message{
			{
				Sender:   "User",
				Receiver: base.name,
				Content:  "hello, my name is bobby",
				Type:     "Msg",
			},
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", run.Messages[0])
	run, err = base.Run(context.Background(),
		[]schema.Message{
			{
				Sender:   "User",
				Receiver: base.name,
				Content:  "what is my name?",
				Type:     "Msg",
			},
		})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", run.Messages[0])
}

func TestGoGraphviz(t *testing.T) {
	path := "test.dot"
	g, err := graphviz.ParseFile(path)
	if err != nil {
		panic(err)
	}
	curNode := g.FirstNode()
	for {
		if curNode == nil {
			break
		}
		fmt.Println(curNode.Name())
		fmt.Println(curNode.Get("label"))
		curNode = g.NextNode(curNode)
	}
}

func TestMCPAgent(t *testing.T) {
	tools, _ := mcp.New(`{
  "mcpServers": {
    "sqlite": {
      "command": "/Users/tyloafer/.local/bin/uvx",
      "args": ["mcp-server-sqlite", "--db-path", "/Users/tyloafer/WorkPlace/ali/python-sdk/examples/clients/simple-chatbot/mcp_simple_chatbot/test.db"]
    }
  }
}`)
	base, err := NewBaseAgent(
		WithLLM(client()),
		WithName("test"),
		WithDesc("test"),
		WithTools(tools),
		WithFeedbacks(&feedback.ContentFeedback{}))
	if err != nil {
		log.Fatal(err)
	}
	run, err := base.Run(context.Background(), []schema.Message{
		{
			Sender:   "User",
			Receiver: base.name,
			Content:  "帮我创建一个学生表，并分别写入三个学生，小明，19岁，81分，小红，19岁，80分，小军，20岁，60分",
			Type:     "Msg",
		},
	},
		llm.WithTemperature(0.1),
		llm.WithTopP(0.8),
		llm.WithRepetitionPenalty(1.05),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", run)
}
