package main

import (
	"context"
	"fmt"

	"github.com/antgroup/aievo/callback"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
)

type CallbackHandler struct {
}

var _ callback.Handler = &CallbackHandler{}

func (h *CallbackHandler) HandleSOP(ctx context.Context, sop string) {
}

func (h *CallbackHandler) HandleLLMStart(ctx context.Context, prompt string) {
	//fmt.Println("LLM Start with prompt:", prompt)
}

func (h *CallbackHandler) HandleLLMEnd(ctx context.Context, output *llm.Generation) {
	// fmt.Println("LLM End with resp ", output.Text, "with cost token ", output.Usage.TotalTokens)
}

func (h *CallbackHandler) HandleAgentStart(ctx context.Context, a schema.Agent, messages []schema.Message) {
	// fmt.Println("Agent start, inputs ", formatMap(inputs))

}

func (h *CallbackHandler) HandleAgentEnd(ctx context.Context, a schema.Agent, result *schema.Generation) {
	// fmt.Println("Agent end, output ", formatMap(outputs))
}

func (h *CallbackHandler) HandleAgentActionStart(ctx context.Context, agent string, action *schema.StepAction) {
}

func (h *CallbackHandler) HandleAgentActionEnd(ctx context.Context, agent string, action *schema.StepAction) {
	scratchpad := fmt.Sprintf(`\nAgent: %s
Thought: %s
Action: %s
Action Input: %s
Observation: %s`, agent, action.Thought, action.Action,
		action.Input, action.Observation)
	fmt.Println(scratchpad)
	fmt.Println("===========================================================================")
}

func (h *CallbackHandler) HandleRetrieverStart(ctx context.Context, query string) {
	fmt.Println("Retriever start, query:", query)
}

func (h *CallbackHandler) HandleRetrieverEnd(ctx context.Context, query string, documents []schema.Document) {
	fmt.Println("Retriever end, result:", formatDoc(documents))
}

func (h *CallbackHandler) HandleMessageInQueue(ctx context.Context, message *schema.Message) {
	if message.Type == schema.MsgTypeEnd {
		fmt.Println("End Message:")
		fmt.Println("Sender: " + message.Sender)
		fmt.Println("Receiver: " + message.Receiver)
		fmt.Println("Msg: " + message.Content)
		fmt.Println("Type: " + message.Type)
		fmt.Println("===========================================================================")
	}
}

func (h *CallbackHandler) HandleMessageOutQueue(ctx context.Context, message *schema.Message) {
	fmt.Println("\nMessage Out Queue:")
	fmt.Println("Thought: " + message.Thought)
	fmt.Println("Sender: " + message.Sender)
	fmt.Println("Receiver: " + message.Receiver)
	fmt.Println("Msg: " + message.Content)
	fmt.Println("Type: " + message.Type)
	fmt.Println("===========================================================================")
}

func (h *CallbackHandler) HandleStreamingFunc(ctx context.Context, chunk []byte) error {
	//fmt.Print(string(chunk))
	return nil
}

func (h *CallbackHandler) HandleReasoningStreamingFunc(ctx context.Context, chunk []byte) error {
	//fmt.Print(string(chunk))
	return nil
}

func formatDoc(docs []schema.Document) string {
	result := ""
	for i, doc := range docs {
		result += fmt.Sprintf("doc %d, score: %f:\n%s\n",
			i, doc.Score, doc.PageContent)
	}
	return result
}
