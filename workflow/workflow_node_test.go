package workflow

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/aievo/agent"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/llm/openai"
	"github.com/antgroup/aievo/schema"
)

func NewTestLLM01() llm.LLM {
	client, _ := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	return client
}

func NewTestAgent01() schema.Agent {
	assistant, _ := agent.NewBaseAgent(
		agent.WithName("engineer"),
		agent.WithDesc("人工智能助手"),
		agent.WithPrompt("你是一个人工智能助手"),
		agent.WithLLM(NewTestLLM01()),
	)
	return assistant
}

func NewTestAgent02(name, desc, prompt string) schema.Agent {
	assistant, _ := agent.NewBaseAgent(
		agent.WithName(name),
		agent.WithDesc(desc),
		agent.WithPrompt(prompt),
		agent.WithLLM(NewTestLLM01()),
	)
	return assistant
}
func NewTestWorkflow01() *Workflow[DTO, DTO] {

	agentNode1, _ := NewAgentNode(
		WithName("agentNode1"),
		WithAgent(NewTestAgent01()),
		WithNodeInputs("input"),
		WithNodeOutputs("t1", "t2"),
	)

	agentNode2, _ := NewAgentNode(
		WithName("agentNode2"),
		WithAgent(NewTestAgent01()),
		WithNodeInputs("t1"),
		WithNodeOutputs("t3"),
	)

	agentNode3, _ := NewAgentNode(
		WithName("agentNode3"),
		WithAgent(NewTestAgent01()),
		WithNodeInputs("t2"),
		WithNodeOutputs("t4"),
	)

	agentNode4, _ := NewLLMNode(
		WithName("llmNode"),
		WithLLM(NewTestLLM01()),
		WithNodeInputs("t3", "t4"),
		WithNodeOutputs("output"),
	)

	f, _ := NewWorkflow[DTO, DTO](
		WithChannelInput[DTO, DTO]("input"),
		WithChannelOutput[DTO, DTO]("output"),
		WithTransits[DTO, DTO](agentNode1, agentNode2, agentNode3, agentNode4),
	)
	return f
}

func NewTestWorkflow02() *Workflow[DTO, DTO] {

	branchNode1, _ := NewBranchNode(
		WithName("branchNode"),
		WithLLM(NewTestLLM01()),
		WithNodeInputs("input"),
		WithNodeConditionalOutputs(
			ConditionalOutput{
				Condition: "当询问物理相关问题时，使用该分支",
				Output:    "t1",
			},
			ConditionalOutput{
				Condition: "当询问化学相关问题时，使用该分支",
				Output:    "t2",
			},
			ConditionalOutput{
				Condition: "当询问生物相关问题时，使用该分支",
				Output:    "t3",
			},
		),
	)

	agentNode1, _ := NewAgentNode(
		WithName("agentNode1"),
		WithAgent(NewTestAgent02("physics assistant", "物理助手", "你是一个物理助手，可以回答各种物理相关问题。")),
		WithNodeInputs("t1"),
		WithNodeOutputs("t4"),
	)

	agentNode2, _ := NewAgentNode(
		WithName("agentNode2"),
		WithAgent(NewTestAgent02("chemistry assistant", "化学助手", "你是一个化学助手，可以回答各种化学相关问题。")),
		WithNodeInputs("t2"),
		WithNodeOutputs("t5"),
	)

	agentNode3, _ := NewAgentNode(
		WithName("agentNode3"),
		WithAgent(NewTestAgent02("biology assistant", "生物助手", "你是一个生物助手，可以回答各种生物相关问题。")),
		WithNodeInputs("t3"),
		WithNodeOutputs("t6"),
	)

	directNode, _ := NewDirectNode(
		WithName("directNode"),
		WithAgent(NewTestAgent01()),
		WithNodeInputs("t4", "t5", "t6"),
		WithNodeOutputs("output"),
	)

	f, _ := NewWorkflow[DTO, DTO](
		WithChannelInput[DTO, DTO]("input"),
		WithChannelOutput[DTO, DTO]("output"),
		WithTransits[DTO, DTO](branchNode1, agentNode1, agentNode2, agentNode3, directNode),
	)

	return f
}

func TestExecuteWorkflow01(t *testing.T) {
	w := NewTestWorkflow01()
	input := From("你是谁")
	output := w.Execute(context.Background(), &input)
	fmt.Println(Object(*output).(string))
}

func TestExecuteWorkflow02(t *testing.T) {
	w := NewTestWorkflow02()
	input := From("细胞的结构")
	output := w.Execute(context.Background(), &input)
	fmt.Println(Object(*output).(string))
}
