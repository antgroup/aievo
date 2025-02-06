package feedback

import (
	"context"

	"github.com/antgroup/aievo/schema"
)

type Middleware func(context.Context, *LLMFeedback, schema.Agent,
	[]schema.Message, []schema.StepAction,
	[]schema.StepAction, string) bool

// MiddlewareChain connect stage into one stage, when return false, the feedback won't be executed.
func middlewareChain(chains ...Middleware) Middleware {
	return func(ctx context.Context, lf *LLMFeedback, agent schema.Agent,
		messages []schema.Message, actions []schema.StepAction,
		steps []schema.StepAction, prompt string) bool {
		for i := 0; i < len(chains); i++ {
			if !chains[i](ctx, lf, agent, messages,
				actions, steps, prompt) {
				return false
			}
		}
		return true
	}
}
