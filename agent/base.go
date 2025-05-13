package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/antgroup/aievo/callback"
	"github.com/antgroup/aievo/feedback"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

var _ schema.Agent = (*BaseAgent)(nil)

const (
	_jsonParse = "(?s)```json\n(.*?)\n```"
)

type BaseAgent struct {
	name string
	desc string
	role string

	llm llm.LLM
	// tools is a list of the action the agent can do.
	tools           []tool.Tool
	useFunctionCall bool
	env             schema.Environment

	fdChain  feedback.Feedback
	callback callback.Handler
	prompt   prompt.Template

	filterMemoryFunc func([]schema.Message) []schema.Message
	parseOutputFunc  func(string, *llm.Generation) ([]schema.StepAction, []schema.Message, error)

	MaxIterations int
	vars          map[string]string
}

func NewBaseAgent(opts ...Option) (*BaseAgent, error) {
	options := &Options{
		Vars: make(map[string]string),
	}
	option := append(defaultBaseOptions(), opts...)
	for _, opt := range option {
		opt(options)
	}

	p := options.prompt + options.instruction + options.suffix
	if p == "" {
		return nil, schema.ErrMissingPrompt
	}
	if options.name == "" {
		return nil, schema.ErrMissingName
	}
	if options.desc == "" {
		return nil, schema.ErrMissingDesc
	}
	if options.LLM == nil {
		return nil, schema.ErrMissingLLM
	}

	template, err := prompt.NewPromptTemplate(p)
	if err != nil {
		return nil, err
	}
	base := &BaseAgent{
		name: options.name,
		desc: options.desc,
		role: options.role,

		llm:             options.LLM,
		env:             options.Env,
		tools:           options.Tools,
		useFunctionCall: options.useFunctionCall,
		fdChain:         options.FeedbackChain,
		callback:        options.Callback,

		MaxIterations:    options.MaxIterations,
		filterMemoryFunc: options.FilterMemoryFunc,
		parseOutputFunc:  options.ParseOutputFunc,

		prompt: template,
		vars:   options.Vars,
	}
	return base, nil
}

func (ba *BaseAgent) Run(ctx context.Context,
	messages []schema.Message, opts ...llm.GenerateOption) (*schema.Generation, error) {
	steps := make([]schema.StepAction, 0)
	tokens := 0
	if ba.filterMemoryFunc != nil {
		messages = ba.filterMemoryFunc(messages)
	}
	for i := 0; i < ba.MaxIterations; i++ {
		feedbacks, actions, msgs, cost, err := ba.Plan(
			ctx, messages, steps, opts...)
		if err != nil {
			return nil, err
		}
		fd := ""
		for _, sfd := range feedbacks {
			fd += fmt.Sprintf("- %s\n", sfd.Feedback)
		}
		for idx := range actions {
			actions[idx].Feedback = fd
			ba.doAction(ctx, &actions[idx])
		}
		steps = append(steps, actions...)

		tokens += cost
		if len(feedbacks) != 0 {
			for _, msg := range msgs {
				steps = append(steps, schema.StepAction{
					Feedback: fd,
					Log:      msg.Log,
				})
			}
			continue
		}

		if len(actions) == 0 && len(msgs) == 0 {
			steps = append(steps, schema.StepAction{
				Feedback: fd,
				Log:      "",
			})
			continue
		}

		if msgs != nil {
			msgs[0].Token = tokens
			return &schema.Generation{
				Messages:    msgs,
				TotalTokens: tokens,
			}, nil
		}
	}
	return nil, schema.ErrNotFinished
}

func (ba *BaseAgent) Plan(ctx context.Context, messages []schema.Message,
	steps []schema.StepAction, opts ...llm.GenerateOption) (
	[]schema.StepFeedback, []schema.StepAction, []schema.Message, int, error) {
	inputs := make(map[string]any, 10)

	for key, value := range ba.vars {
		inputs[key] = value
	}

	if ba.useFunctionCall {
		opts = append(opts, llm.WithTools(ConvertToolToFunctionDefinition(ba.Tools())))
	} else {
		inputs["tool_names"] = schema.ConvertToolNames(ba.tools)
		inputs["tool_descriptions"] = schema.ConvertToolDescriptions(ba.tools)
	}

	inputs["name"] = ba.name
	inputs["role"] = ba.role
	inputs["history"] = schema.ConvertConstructScratchPad(ba.name, "me", messages, steps)
	inputs["current"] = time.Now().Format("2006-01-02 15:04:05")

	if ba.env != nil {
		inputs["agent_names"] = schema.ConvertAgentNames(ba.env.GetSubscribeAgents(ctx, ba))
		inputs["agent_descriptions"] = schema.ConvertAgentDescriptions(ba.env.GetSubscribeAgents(ctx, ba))
		inputs["sop"] = ba.env.SOP()
	}

	p, err := ba.prompt.Format(inputs)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	if ba.callback != nil {
		ba.callback.HandleLLMStart(ctx, p)
		opts = append(opts, llm.WithStreamingFunc(
			ba.callback.HandleStreamingFunc))
	}

	output, err := ba.llm.Generate(ctx, p, opts...)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	if ba.callback != nil {
		ba.callback.HandleLLMEnd(ctx, output)
	}

	feedbacks := make([]schema.StepFeedback, 0)
	actions, content, err := ba.parseOutputFunc(ba.name, output)
	if err != nil {
		feedbacks = append(feedbacks, schema.StepFeedback{
			Feedback: "parse output failed with error: " + err.Error(),
			Log:      output.Content,
		})
		return feedbacks, actions, content, output.Usage.TotalTokens, nil
	}
	fd := ba.fdChain.Feedback(ctx, ba, content, actions, steps, p)
	if fd.Type == feedback.NotApproved {
		feedbacks = append(feedbacks, schema.StepFeedback{
			Feedback: fd.Msg,
			Log:      output.Content,
		})
	}

	if len(feedbacks) != 0 {
		return feedbacks, actions, content, output.Usage.TotalTokens, nil
	}

	return feedbacks, actions, content, output.Usage.TotalTokens, err
}

func (ba *BaseAgent) doAction(
	ctx context.Context, action *schema.StepAction) {
	var err error
	if ba.callback != nil {
		ba.callback.HandleAgentActionStart(ctx, ba.Name(), action)
	}

	t := ba.getAction(action.Action)
	if t == nil {
		action.Feedback += fmt.Sprintf("- %s is not a valid tool, please check your answer\n", action.Action)
		return
	}

	action.Observation, err = t.Call(ctx, action.Input)
	if err != nil {
		action.Feedback = err.Error()
	}

	if ba.callback != nil {
		ba.callback.HandleAgentActionEnd(ctx, ba.Name(), action)
	}
}

func (ba *BaseAgent) getAction(name string) tool.Tool {
	for _, a := range ba.tools {
		if strings.EqualFold(a.Name(), name) {
			return a
		}
	}
	return nil
}

func ConvertToolToFunctionDefinition(tools []tool.Tool) []llm.Tool {
	convertedTools := make([]llm.Tool, 0)
	for _, t := range tools {
		if t == nil {
			continue
		}

		functionDefinition := &llm.FunctionDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Schema(),
			Strict:      t.Strict(),
		}

		convertedTool := &llm.Tool{
			Type:     "function",
			Function: functionDefinition,
		}
		convertedTools = append(convertedTools, *convertedTool)
	}
	return convertedTools
}

func parseOutput(name string, output *llm.Generation) ([]schema.StepAction, []schema.Message, error) {
	if len(output.ToolCalls) > 0 {
		return parseToolCalls(output.ToolCalls), nil, nil
	}
	content := strings.TrimSpace(output.Content)
	if content == "" {
		return nil, nil, errors.New("content is empty")
	}
	content = extractJSONContent(content)
	action, err := parseAction(content)
	if err != nil {
		return nil, nil, err
	}
	if action != nil {
		return []schema.StepAction{*action}, nil, nil
	}
	message, err := parseMessage(name, content)
	if err != nil {
		return nil, nil, err
	}
	return nil, []schema.Message{*message}, nil
}

func parseToolCalls(toolCalls []llm.ToolCall) []schema.StepAction {
	actions := make([]schema.StepAction, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		logBytes, _ := json.Marshal(toolCall)
		action := schema.StepAction{
			Action: toolCall.Function.Name,
			Input:  toolCall.Function.Arguments,
			Log:    string(logBytes),
		}
		actions = append(actions, action)
	}
	return actions
}

func extractJSONContent(content string) string {
	compile := regexp.MustCompile(_jsonParse)
	submatch := compile.FindAllStringSubmatch(content, -1)
	if len(submatch) > 0 {
		return strings.TrimSpace(submatch[0][1])
	}
	return content
}

func parseAction(content string) (*schema.StepAction, error) {
	action := &schema.StepAction{}
	// fix: action input may be json instead of json string
	actionInput := &schema.StepActionInput{}
	onlyAction := &schema.StepOnlyAction{}
	if err := json.Unmarshal([]byte(content), actionInput); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(content), onlyAction); err != nil {
		return nil, err
	}
	action.Action = onlyAction.Action
	action.Id = onlyAction.Id
	action.Thought = onlyAction.Thought
	action.Log = content

	switch actionInput.Input.(type) {
	case string:
		action.Input = actionInput.Input.(string)
	default:
		marshal, _ := json.Marshal(actionInput.Input)
		action.Input = string(marshal)
	}
	if action.Action != "" {
		return action, nil
	}
	return nil, nil
}

func parseMessage(name, content string) (*schema.Message, error) {
	message := &schema.Message{Log: content, Sender: name}
	if err := json.Unmarshal([]byte(content), message); err != nil {
		return nil, err
	}
	return message, nil
}

func (ba *BaseAgent) Name() string {
	return ba.name
}

func (ba *BaseAgent) Description() string {
	return ba.desc
}

func (ba *BaseAgent) WithEnv(env schema.Environment) {
	ba.env = env
}

func (ba *BaseAgent) Env() schema.Environment {
	return ba.env
}

func (ba *BaseAgent) Tools() []tool.Tool {
	return ba.tools
}
