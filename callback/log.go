package callback

import (
	"context"
	"fmt"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
)

type LogHandler struct{}

func (h LogHandler) HandleSOP(ctx context.Context, sop string) {
	fmt.Println("SOP: ", sop)
}

func (h LogHandler) HandleLLMEnd(ctx context.Context, output *llm.Generation) {
}

func (LogHandler) HandleLLMStart(ctx context.Context, prompt string) {
}

func (LogHandler) HandleAgentStart(ctx context.Context, _ schema.Agent, _ []schema.Message) {

}

func (LogHandler) HandleAgentEnd(ctx context.Context, _ schema.Agent, _ *schema.Generation) {
}

func (LogHandler) HandleAgentActionStart(ctx context.Context, agent string, action *schema.StepAction) {
}

func (LogHandler) HandleAgentActionEnd(ctx context.Context, agent string, action *schema.StepAction) {
	fmt.Printf("Agent: %s\nThought: %s\nAction: %s\n"+
		"Action Input: %s\nObservation: %s", agent, action.Thought, action.Action,
		action.Input, action.Observation)
}

func (LogHandler) HandleRetrieverStart(ctx context.Context, query string) {
	fmt.Println("Retriever start, query:", query)
}

func (LogHandler) HandleRetrieverEnd(ctx context.Context, query string, documents []schema.Document) {
	fmt.Println("Retriever end, result:", formatDoc(documents))
}

func (LogHandler) HandleMessageInQueue(ctx context.Context, message *schema.Message) {
	if message.Type == schema.MsgTypeCreative {
		if message.MngInfo != nil && len(message.MngInfo.Remove) != 0 {
			fmt.Println("(Watcher)Remove:", message.MngInfo.Remove)
			return
		}
		fmt.Println("(Watcher): Do Nothing")
		return
	}
	if message.Type == schema.MsgTypeEnd {
		fmt.Printf("Final Answer: %s\n", message.Content)
		return
	}
	if message.IsMsg() {
		fmt.Printf("(%s -> %s)(%s): %s\n", message.Sender, message.Receiver, message.Condition, message.Content)
	}
}

func (LogHandler) HandleMessageOutQueue(ctx context.Context, message *schema.Message) {
}

func (LogHandler) HandleStreamingFunc(ctx context.Context, chunk []byte) error {
	return nil
}

func (LogHandler) HandleReasoningStreamingFunc(ctx context.Context, chunk []byte) error {
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
