package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/antgroup/aievo/driver"
	"github.com/antgroup/aievo/feedback"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

var _ schema.Agent = (*GraphAgent)(nil)

type GraphAgent struct {
	BaseAgent
	sop    string
	Driver driver.Driver
}

func NewGraphAgent(opts ...Option) (*GraphAgent, error) {
	options := &Options{
		Vars: make(map[string]string),
	}
	option := append(defaultGraphOptions(), opts...)
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

	if options.Env != nil && options.Env.SOP() != "" &&
		options.SOPGraph == "" {
		options.SOPGraph = options.Env.SOP()
	}
	if options.SOPGraph == "" {
		return nil, schema.ErrMissingGraph
	}

	template, err := prompt.NewPromptTemplate(p)
	if err != nil {
		return nil, err
	}

	dri, err := options.Driver.InitGraph(context.Background(),
		options.SOPGraph)
	if err != nil {
		return nil, err
	}
	base := &GraphAgent{
		BaseAgent: BaseAgent{
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
		},
		sop:    options.SOPGraph,
		Driver: dri,
	}

	return base, nil
}

func (ba *GraphAgent) InitGraph(ctx context.Context) error {
	var err error
	if ba.sop == "" && ba.Env() != nil && ba.Env().SOP() != "" {
		ba.sop = ba.Env().SOP()
	}
	if !ba.Driver.IsInit() {
		if ba.Env() == nil || ba.Env().SOP() == "" {
			return errors.New("driver is not initialized, cannot find graph sop")
		}
		ba.Driver, err = ba.Driver.InitGraph(ctx, ba.Env().SOP())
		if err != nil {
			return errors.New("driver is not initialized, graph sop parse failed, err: " + err.Error())
		}
	}
	return nil
}

func (ba *GraphAgent) Run(ctx context.Context,
	messages []schema.Message, opts ...llm.GenerateOption) (*schema.Generation, error) {
	// 初始化graph, 避免graph是由env传入的
	err := ba.InitGraph(ctx)
	if err != nil {
		return nil, err
	}
	steps := make([]schema.StepAction, 0)
	tokens := 0
	if ba.filterMemoryFunc != nil {
		messages = ba.filterMemoryFunc(messages)
	}
	for i := 0; i < ba.MaxIterations; i++ {
		// 获取当前已经执行的graph
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

		// todo: feedback校验是否在graph里面
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
		// 更新graph 状态
		ba.Driver.UpdateGraphState(ctx, steps, actions)
	}
	return nil, schema.ErrNotFinished
}

func (ba *GraphAgent) Plan(ctx context.Context, messages []schema.Message,
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

	inputs["name"] = ba.Name()
	inputs["role"] = ba.role
	inputs["history"] = schema.ConvertConstructScratchPad(ba.name, "me", messages, nil)
	inputs["current"] = time.Now().Format("2006-01-02 15:04:05")

	inputs["current_nodes"], inputs["next_nodes"],
		inputs["all_nodes"] = ba.Driver.RenderStates()

	inputs["sop"] = ba.sop
	inputs["current_sop"], _ = ba.Driver.RenderCurrentGraph()
	if ba.env != nil {
		inputs["agent_names"] = schema.ConvertAgentNames(ba.env.GetSubscribeAgents(ctx, ba))
		inputs["agent_descriptions"] = schema.ConvertAgentDescriptions(ba.env.GetSubscribeAgents(ctx, ba))
	}
	fmt.Println("current nodes")
	fmt.Println(inputs["current_nodes"])
	fmt.Println("next_nodes")
	fmt.Println(inputs["next_nodes"])
	fmt.Println("all nodes")
	fmt.Println(inputs["all_nodes"])
	fmt.Println("tmp render graph")
	fmt.Println(ba.Driver.TmpRender())

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

func (ba *GraphAgent) doAction(
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

func (ba *GraphAgent) getAction(name string) tool.Tool {
	for _, a := range ba.tools {
		if strings.EqualFold(a.Name(), name) {
			return a
		}
	}
	return nil
}

func (ba *GraphAgent) Name() string {
	return ba.name
}

func (ba *GraphAgent) Description() string {
	return ba.desc
}

func (ba *GraphAgent) WithEnv(env schema.Environment) {
	ba.env = env
}

func (ba *GraphAgent) Env() schema.Environment {
	return ba.env
}

func (ba *GraphAgent) Tools() []tool.Tool {
	return ba.tools
}

func parseGraphOutput(name string, output *llm.Generation) ([]schema.StepAction, []schema.Message, error) {
	if len(output.ToolCalls) > 0 {
		return parseToolCalls(output.ToolCalls), nil, nil
	}
	content := strings.TrimSpace(output.Content)
	if content == "" {
		return nil, nil, errors.New("content is empty")
	}
	content = json.TrimJsonString(content)
	actions, _ := parseActionArray(content)
	if len(actions) != 0 {
		return actions, nil, nil
	}
	action, _ := parseAction(content)
	if action != nil {
		return []schema.StepAction{*action}, nil, nil
	}
	message, err := parseMessage(name, content)
	if err != nil {
		return nil, nil, err
	}
	return nil, []schema.Message{*message}, nil
}

func parseActionArray(content string) ([]schema.StepAction, error) {
	actions := make([]schema.StepAction, 0)
	actionInputs := make([]schema.StepActionInput, 0)
	// fix: action input may be json instead of json string
	if err := json.Unmarshal([]byte(content), &actions); err != nil {
		for _, stepAction := range actions {
			if stepAction.Action == "" {
				return nil, err
			}
		}
	}
	if err := json.Unmarshal([]byte(content), &actionInputs); err != nil {
		return nil, err
	}

	for i, actionInput := range actionInputs {
		switch actionInput.Input.(type) {
		case string:
			actions[i].Input = actionInput.Input.(string)
		default:
			marshal, _ := json.Marshal(actionInput.Input)
			actions[i].Input = string(marshal)
		}
		actions[i].Log = content
	}

	return actions, nil
}
