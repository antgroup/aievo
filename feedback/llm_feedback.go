package feedback

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/utils/json"
	"github.com/antgroup/aievo/utils/parallel"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
)

// when use llm feedback, please set template instead of default template
const _defaultTemplate = `
# Goal
Here is your goal and how to do it

# Context
## Self
Your name is {{.name}} and your description is {{.description}}

## Team Member
{{.agent_descriptions}}

## Tools
{{.tool_descriptions}}

## SOP
{{.sop}}

## History
{{.history}}

# Response Format
Your final output must in the following format:
{
	"type": "Approved/NotApproved",
	"msg": "When You are not approved, you should give your suggestion"
}
`

type LLMFeedback struct {
	llm              llm.LLM
	tpl              *prompt.PromptTemplate
	prompt           string
	maxcb            int
	curturn          int
	expert           int
	middlewaresChain Middleware
	Vars             map[string]any
}

func NewLLMFeedback(LLM llm.LLM, opts ...LLMFeedbackOption) (Feedback, error) {
	if LLM == nil {
		return nil, schema.ErrMissingLLM
	}
	fd := &LLMFeedback{
		llm:    LLM,
		prompt: _defaultTemplate,
		expert: 1,
		maxcb:  3,
	}
	for _, opt := range opts {
		opt(fd)
	}

	tpl, err := prompt.NewPromptTemplate(fd.prompt)
	if err != nil {
		return nil, schema.ErrParsePromptTemplate
	}
	fd.tpl = tpl
	return fd, nil
}

func (lf *LLMFeedback) Feedback(ctx context.Context, agent schema.Agent,
	messages []schema.Message, actions []schema.StepAction,
	steps []schema.StepAction, prompt string) *FeedbackInfo {
	if lf.middlewaresChain != nil &&
		!lf.middlewaresChain(ctx, lf, agent, messages, actions, steps, prompt) {
		return &FeedbackInfo{Type: Approved}
	}

	info := lf.feedback(ctx, agent, messages, actions, steps, prompt)

	if info.Type == Approved {
		lf.curturn = 0
	} else {
		lf.curturn++
	}
	return info
}

func (lf *LLMFeedback) feedback(ctx context.Context, agent schema.Agent,
	messages []schema.Message, actions []schema.StepAction,
	steps []schema.StepAction, _ string) *FeedbackInfo {

	// 考虑一直反馈失败的情况
	info := &FeedbackInfo{Type: Approved}
	if lf.maxcb > 0 && lf.curturn >= lf.maxcb {
		return info
	}

	vars := make(map[string]any)
	if agent.Env() != nil {
		memory := agent.Env().LoadMemory(ctx, agent)
		vars["history"] = schema.ConvertConstructScratchPad(agent.Name(),
			agent.Name(), memory, steps)
		vars["sop"] = agent.Env().SOP()
		vars["agent_names"] = schema.ConvertAgentNames(agent.Env().GetSubscribeAgents(ctx, agent))
		vars["agent_descriptions"] = schema.ConvertAgentDescriptions(agent.Env().GetSubscribeAgents(ctx, agent))
	}

	vars["tool_names"] = schema.ConvertToolNames(agent.Tools())
	vars["tool_descriptions"] = schema.ConvertToolDescriptions(agent.Tools())
	vars["name"] = agent.Name()
	vars["description"] = agent.Description()

	if len(actions) != 0 {
		vars["response"] = actions[0].Log
	}
	if len(messages) != 0 {
		vars["response"] = messages[0].Log
	}

	for k, v := range lf.Vars {
		vars[k] = v
	}

	p, err := lf.tpl.Format(vars)
	// 默认通过，避免阻塞
	if err != nil {
		return info
	}

	approve := int32(0)
	parallel.Parallel(func(i int) any {
		temperatures := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
		result, err := lf.llm.Generate(ctx, p,
			llm.WithTemperature(temperatures[i%len(temperatures)]))
		if err != nil {
			atomic.AddInt32(&approve, 1)
			return nil
		}
		info.Token = result.Usage.TotalTokens
		tmp := &FeedbackInfo{Type: Approved}
		_ = json.Unmarshal([]byte(json.TrimJsonString(result.Content)), tmp)
		if tmp == nil || tmp.Type == Approved {
			atomic.AddInt32(&approve, 1)
		} else {
			info.Msg += strings.TrimSpace(tmp.Msg) + "\n"
		}
		return tmp
	}, lf.expert, lf.expert)

	info.Type = NotApproved
	if approve > int32(lf.expert/2) {
		info.Type = Approved
		info.Msg = ""
	}

	return info
}
