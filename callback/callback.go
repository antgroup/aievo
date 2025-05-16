package callback

import (
	"context"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
)

type Handler interface {
	HandleSOP(ctx context.Context, sop string)
	HandleLLMStart(ctx context.Context, prompt string)
	HandleLLMEnd(ctx context.Context, output *llm.Generation)
	HandleAgentStart(ctx context.Context, a schema.Agent, messages []schema.Message)
	HandleAgentEnd(ctx context.Context, a schema.Agent, result *schema.Generation)
	HandleAgentActionStart(ctx context.Context, agent string, action *schema.StepAction)
	HandleAgentActionEnd(ctx context.Context, agent string, action *schema.StepAction)
	HandleRetrieverStart(ctx context.Context, query string)
	HandleRetrieverEnd(ctx context.Context, query string, documents []schema.Document)
	HandleMessageInQueue(ctx context.Context, message *schema.Message)
	HandleMessageOutQueue(ctx context.Context, message *schema.Message)
	HandleStreamingFunc(ctx context.Context, chunk []byte) error
	HandleReasoningStreamingFunc(ctx context.Context, chunk []byte) error
}
