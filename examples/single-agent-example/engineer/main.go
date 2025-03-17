package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/tool/bash"
	"github.com/antgroup/aievo/tool/file"
)

func main() {
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}
	workspace, _ := os.Getwd()
	workspace = filepath.Join(workspace,
		"examples", "single-agent-example", "engineer", "workspace")
	// 文件创建 文件读取 文件修改 文件删除 文件重命名
	// 文件夹创建 文件夹读取 文件夹删除 文件夹重命名
	fileTools, _ := file.GetFileRelatedTools(workspace)
	// 命令执行
	bashTool, _ := bash.New()

	callbackHandler := &CallbackHandler{}

	engineerTools := make([]tool.Tool, 0)
	engineerTools = append(engineerTools, fileTools...)
	engineerTools = append(engineerTools, bashTool)

	engineer, err := agent.NewBaseAgent(
		agent.WithName("engineer"),
		agent.WithDesc(EngineerDescription),
		agent.WithPrompt(EngineerPrompt),
		agent.WithInstruction(SingleAgentInstructions),
		agent.WithVars("sop", Workflow),
		agent.WithVars("workspace", workspace),
		agent.WithTools(engineerTools),
		agent.WithLLM(client),
		agent.WithCallback(callbackHandler),
	)
	if err != nil {
		log.Fatal(err)
	}

	gen, err := engineer.Run(context.Background(), []schema.Message{
		{
			Type:     schema.MsgTypeMsg,
			Content:  "使用pyqt写一个贪吃蛇",
			Sender:   "User",
			Receiver: "engineer",
		},
	}, llm.WithTemperature(0.1))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(gen.Messages[0].Content)
}
