package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/aievo"
	"github.com/antgroup/aievo/environment"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/memory"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/tool/arxiv"
	"github.com/antgroup/aievo/tool/search"
)

const searchApiKey = "xxx"

func main() {
	// 大模型实例化
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}
	// 实例化所需要的Tools
	// 搜索引擎
	search, _ := search.New(
		search.WithEngine("google"),
		search.WithApiKey(searchApiKey),
		search.WithTopK(3),
	)

	// 论文检索
	arxiv, _ := arxiv.New(
		arxiv.WithTopk(10),
		arxiv.WithUserAgent("aievo/1.0"),
	)

	callbackHandler := &CallbackHandler{}

	// 实例化Agents
	// RTA
	RTA, _ := agent.NewBaseAgent(
		agent.WithName("RTA"),
		agent.WithDesc(RTADescription),
		agent.WithPrompt(RTAPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
	)

	// LROA
	LROA, _ := agent.NewBaseAgent(
		agent.WithName("LROA"),
		agent.WithDesc(LROADescription),
		agent.WithPrompt(LROAPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithTools([]tool.Tool{search, arxiv}),
		agent.WithCallback(callbackHandler),
	)

	// OGA
	OGA, _ := agent.NewBaseAgent(
		agent.WithName("OGA"),
		agent.WithDesc(OGADescription),
		agent.WithPrompt(OGAPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
		agent.WithFilterMemoryFunc(func(msgs []schema.Message) []schema.Message {
			haveCGA := false
			for _, msg := range msgs {
				if msg.Receiver == "CGA" {
					haveCGA = true
					break
				}
			}
			if haveCGA {
				// 如果和CGA交互过了, 删除所有和LROA的交互记忆
				filterMsgs := make([]schema.Message, 0)
				for _, msg := range msgs {
					if msg.Sender == "LROA" || msg.Receiver == "LROA" {
						continue
					}
					filterMsgs = append(filterMsgs, msg)
				}
				msgs = filterMsgs
			}
			filterMsgs := make([]schema.Message, 0)
			for _, msg := range msgs {
				if msg.Sender == "CGA" && msg.Receiver == "OGA" {
					// 去除换行符
					msg.Content = strings.ReplaceAll(msg.Content, "\n", " ")
				}
				filterMsgs = append(filterMsgs, msg)
			}
			msgs = filterMsgs
			return msgs
		}),
	)

	// CGA
	CGA, _ := agent.NewBaseAgent(
		agent.WithName("CGA"),
		agent.WithDesc(CGADescription),
		agent.WithPrompt(CGAPrompt),
		agent.WithInstruction(defaultBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
		agent.WithFilterMemoryFunc(func(msgs []schema.Message) []schema.Message {
			// 仅保留最后一个msg
			return msgs[len(msgs)-1:]
		}),
	)

	// PPA
	PPA, _ := agent.NewBaseAgent(
		agent.WithName("PPA"),
		agent.WithDesc(PPADescription),
		agent.WithPrompt(PPAPrompt),
		agent.WithInstruction(defaultEndBaseInstructions),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
	)

	env := environment.NewEnv()

	env.Memory = memory.NewBufferMemory()

	team := make([]schema.Agent, 0)
	team = append(team, RTA, LROA, OGA, CGA, PPA)

	opts := make([]aievo.Option, 0)
	opts = append(opts,
		aievo.WithTeam(team),
		aievo.WithMaxTurn(50),
		aievo.WithCallback(callbackHandler),
		aievo.WithLLM(client),
		aievo.WithEnvironment(env),
		aievo.WithTeamLeader(RTA),
		aievo.WithSOP(workflow),
		aievo.WithUserProxy(nil),
	)

	evo, err := aievo.NewAIEvo(opts...)
	if err != nil {
		panic(err)
	}
	run, err := evo.Run(context.Background(),
		"MultiAIAgent/MultiAgent 高效协作",
		llm.WithTemperature(0.1),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(run)
}
