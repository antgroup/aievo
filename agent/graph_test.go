package agent

import (
	"context"
	"testing"

	"github.com/antgroup/aievo/environment"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool/mcp"
)

func TestGraph(t *testing.T) {

	tools, err := mcp.New(`
{
  "mcpServers": {
    "jina-mcp-tools": {
      "command": "npx",
      "args": ["jina-mcp-tools"],
      "env": {
        "JINA_API_KEY": "jina_xxxx"
      }
    },
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        ".",
      ]
    }
  }
}
`)
	sopGraph := `
digraph TechDocGeneration {
    node [style=filled, shape=rect, fontname="Arial"]
    edge [fontname="Arial", fontsize=10]

    /* 初始化阶段 */
    RequirementAnalysis [fillcolor=white, label="需求分析\n(确定文档范围)"]

    /* 技术检索流程 */
    TechResearch [fillcolor=white, label="技术检索\n(API/文档收集)"]

    /* 目录生成流程 */
    TOCGeneration [fillcolor=white, label="目录生成", shape=folder]

    /* 章节编写 */
    Chap1 [fillcolor=white, label="1. 概述"]
    Chap2 [fillcolor=white, label="2. 架构"]
    Chap3 [fillcolor=white, label="3. 实现"]
    Chap4 [fillcolor=white, label="4. 示例"]

    /* 收尾流程 */
    DocAssembly [fillcolor=white, label="文档整合"]

    /* 主流程连接 */
    RequirementAnalysis -> TechResearch -> TOCGeneration
    TOCGeneration -> Chap1
    TOCGeneration -> Chap2
    TOCGeneration -> Chap3
    TOCGeneration -> Chap4
    
    Chap1 -> DocAssembly
    Chap2 -> DocAssembly
    Chap3 -> DocAssembly
    Chap4 -> DocAssembly
}
`
	if err != nil {
		t.Fatal(err)
	}
	env := environment.NewEnv()
	env.Sop = sopGraph
	sop, err := NewGraphAgent(WithLLM(client()),
		WithName("yu"),
		WithDesc("an intelligent assistant"),
		WithTools(tools),
		WithEnv(env),
		// WithSOPGraph(sopGraph),
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := sop.Run(context.Background(), []schema.Message{
		{
			Type: schema.MsgTypeMsg,
			Content: `帮我写一篇关于MCP协议(modelcontextprotocol)的技术分享文章
面向用户：开发者
需要包含的内容：协议介绍、协议分析、架构分析、demo等
参考文档：mcp官方文档

每一章节，你应该尽量详细，并为每章的内容创建一个文件并写入
最后，汇总每章文件的内容，生成一个最终的技术文档给我`,
		}})
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log(result)
}

func TestParseGraphOutput(t *testing.T) {
	generation := llm.Generation{Content: `
{
  "thought" : "Based on the research, I'll generate a detailed table of contents that covers all required sections (protocol introduction, analysis, architecture, and demo) while incorporating the key information we've gathered.",
  "action" : "write_file",
  "input" : {
    "path" : "mcp_toc.md",
    "content" : "# Table of Contents for MCP Protocol Technical Document\n\n## 1. Introduction to Model Context Protocol (MCP)\n   - 1.1 What is MCP?\n   - 1.2 Why MCP? (Standardization benefits)\n   - 1.3 Key Concepts and Terminology\n\n## 2. Protocol Analysis\n   - 2.1 Base Protocol (JSON-RPC foundation)\n   - 2.2 Core Features\n     - Resources\n     - Prompts\n     - Tools\n     - Sampling\n   - 2.3 Security and Trust Considerations\n\n## 3. Architecture Analysis\n   - 3.1 System Components\n     - Hosts\n     - Clients\n     - Servers\n   - 3.2 Communication Flow\n   - 3.3 Data Access Patterns\n\n## 4. Practical Implementation and Demo\n   - 4.1 Setting Up an MCP Environment\n   - 4.2 Building a Simple MCP Server\n   - 4.3 Creating an MCP Client\n   - 4.4 Example Use Cases\n\n## 5. Additional Resources\n   - Official Documentation\n   - SDKs and Libraries\n   - Community Resources"
  },
  "node" : "TOCGeneration"
}`}
	actions, _, err := parseGraphOutput("test", &generation)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) == 0 {
		t.Fatal("no actions")
	}
}
