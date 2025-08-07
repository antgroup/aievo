package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antgroup/aievo/callback"
	"github.com/antgroup/aievo/feedback"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/tool/filereaders"
	"github.com/antgroup/aievo/utils/json"
)

var _ schema.Agent = (*BaseAgent)(nil)

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
	totalFeedbacks := 0
	if ba.filterMemoryFunc != nil {
		messages = ba.filterMemoryFunc(messages)
	}
	for i := 0; i < ba.MaxIterations; i++ {
		if totalFeedbacks > 5 {
			steps = make([]schema.StepAction, 0) // 清空steps
			totalFeedbacks = 0                   // 重置feedback计数器
		}
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
			totalFeedbacks += len(feedbacks)
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
	inputs["description"] = ba.desc
	inputs["role"] = ba.role
	inputs["history"] = schema.ConvertConstructScratchPad(ba.name, "me", messages, steps)
	inputs["current"] = time.Now().Format("2006-01-02 15:04:05")

	if ba.env != nil {
		inputs["agent_names"] = schema.ConvertAgentNames(ba.env.GetSubscribeAgents(ctx, ba))
		inputs["agent_descriptions"] = schema.ConvertAgentDescriptions(ba.env.GetSubscribeAgents(ctx, ba))
		inputs["sop"] = ba.env.SOP()
	}

	if strings.Contains(ba.name, "File") || strings.Contains(ba.name, "file") {
		// Extract the question from the first message
		question := messages[0].Content
		// If the question contains a filename, extract it
		if strings.Contains(question, "FILENAME:") {
			parts := strings.Split(question, "FILENAME:")
			filename := strings.TrimSpace(parts[1])
			// Extract until next whitespace or end of string
			if idx := strings.Index(filename, " "); idx > 0 {
				filename = filename[:idx]
			}
			// Construct the full file path using relative path
			fullPath := filepath.Join("../../../dataset", "gaia", "val_files", filename)
			// Create file reader and read the file
			reader := filereaders.NewGeneralReader()
			fileContent, err := reader.Read("read file content", fullPath)
			if err != nil {
				inputs["file"] = fmt.Sprintf("Error reading file %s: %v", filename, err)
			} else {
				inputs["file"] = fileContent
			}
		} else {
			inputs["file"] = "No file provided"
		}
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
	// print output content
	//fmt.Printf("LLM Output: %s\n", output.Content)
	// Clean up any thinking content by removing everything before "</think>"
	if strings.Contains(output.Content, "</think>") {
		parts := strings.Split(output.Content, "</think>")
		if len(parts) > 1 {
			output.Content = parts[1]
		}
	}
	// 记录输入输出
	logfile := fmt.Sprintf("eval/log_level_%s.log", time.Now().Format("2006-0102"))
	// Open log file in append mode
	f, err := os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Just log to stderr if file can't be opened
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
	} else {
		defer f.Close()
		// 记录每次的输入与输出
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(f, "[%s]=====\n===Prompt:\n%s\n===Output:\n%s\n\n", timestamp, p, output.Content)

		// 只记录上一条历史和模型输出
		//var historyLog string
		//if len(steps) == 0 {
		//	var sb strings.Builder
		//	msg := messages[len(messages)-1]
		//	sb.WriteString(fmt.Sprintf("(%s -> %s): %s\n", msg.Sender, msg.Receiver, msg.Content))
		//	historyLog = sb.String()
		//} else {
		//	lastStep := steps[len(steps)-1]
		//	if lastStep.Observation != "" {
		//		historyLog = fmt.Sprintf("Observation: %s", lastStep.Observation)
		//	} else {
		//		historyLog = fmt.Sprintf("Feedback: %s", lastStep.Feedback)
		//	}
		//}
		//fmt.Fprintf(f, "History: %s\nOutput of %s:\n%s\n\n",
		//	historyLog, ba.name, output.Content)
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
	content = json.TrimJsonString(content)

	// 如果是消息列表
	if content[0] == '[' && content[len(content)-1] == ']' {
		messages, err := parseMessageList(name, content)
		if err != nil {
			return nil, nil, err
		}
		if len(messages) == 0 {
			return nil, nil, errors.New("no valid messages found")
		}
		return nil, messages, nil
	}

	// 如果是单条消息
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

func parseAction(content string) (*schema.StepAction, error) {
	action := &schema.StepAction{Log: content}
	// fix: action input may be json instead of json string
	actionInput := &schema.StepActionInput{}
	if err := json.Unmarshal([]byte(content), action); err != nil {
		if action.Action == "" {
			return nil, err
		}
	}
	if err := json.Unmarshal([]byte(content), actionInput); err != nil {
		return nil, err
	}

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

	// 先解析为map来处理content字段可能是对象的情况
	var rawMessage map[string]interface{}
	if err := json.Unmarshal([]byte(content), &rawMessage); err != nil {
		return nil, err
	}

	// 检查是否包含必需的 'cate' 字段
	if cateValue, exists := rawMessage["cate"]; !exists {
		return nil, errors.New("message content missing required 'cate' field")
	} else {
		if cateStr, ok := cateValue.(string); ok {
			if cateStr != "MSG" && cateStr != "Msg" && cateStr != "msg" && cateStr != "end" && cateStr != "End" && cateStr != "END" {
				rawMessage["cate"] = "MSG"
				if _, receiverExists := rawMessage["receiver"]; !receiverExists {
					return nil, errors.New("field 'receiver' is required")
				}
			}
		} else {
			return nil, errors.New("'cate' field must be a string")
		}
	}

	// 如果content字段是对象，将其序列化为字符串
	if contentObj, exists := rawMessage["content"]; exists {
		if contentStr, ok := contentObj.(string); ok {
			// 如果已经是字符串，保持不变
			rawMessage["content"] = contentStr
		} else {
			// 如果是对象，序列化为JSON字符串
			contentBytes, err := json.Marshal(contentObj)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize content object: %w", err)
			}
			rawMessage["content"] = string(contentBytes)
		}
	}

	// 重新序列化并解析为Message结构
	processedBytes, err := json.Marshal(rawMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal processed message: %w", err)
	}

	if err := json.Unmarshal(processedBytes, message); err != nil {
		return nil, err
	}

	return message, nil
}

func parseMessageList(name, content string) ([]schema.Message, error) {
	// 首先尝试解析为JSON数组
	var jsonArray []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &jsonArray); err != nil {
		return nil, fmt.Errorf("failed to parse content as JSON array: %w", err)
	}

	messages := make([]schema.Message, 0, len(jsonArray))
	for i, jsonObj := range jsonArray {
		// 检查每个JSON对象是否包含必需的 'cate' 字段
		if cateValue, exists := jsonObj["cate"]; !exists {
			return nil, fmt.Errorf("message at index %d missing required 'cate' field", i)
		} else {
			if cateStr, ok := cateValue.(string); ok {
				if cateStr != "MSG" && cateStr != "END" {
					jsonObj["cate"] = "MSG"
					if _, receiverExists := jsonObj["receiver"]; !receiverExists {
						return nil, fmt.Errorf("field 'receiver' is required for message at index %d when 'cate' is not 'MSG' or 'END'", i)
					}
				}
			} else {
				return nil, fmt.Errorf("'cate' field must be a string for message at index %d", i)
			}
		}

		// 如果content字段是对象，将其序列化为字符串
		if contentObj, exists := jsonObj["content"]; exists {
			if contentStr, ok := contentObj.(string); ok {
				// 如果已经是字符串，保持不变
				jsonObj["content"] = contentStr
			} else {
				// 如果是对象，序列化为JSON字符串
				contentBytes, err := json.Marshal(contentObj)
				if err != nil {
					return nil, fmt.Errorf("failed to serialize content object at index %d: %w", i, err)
				}
				jsonObj["content"] = string(contentBytes)
			}
		}

		// 将每个JSON对象转换回字节数组
		objBytes, err := json.Marshal(jsonObj)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal object at index %d: %w", i, err)
		}

		// 解析为Message结构
		message := &schema.Message{Log: string(objBytes), Sender: name}
		if err := json.Unmarshal(objBytes, message); err != nil {
			return nil, fmt.Errorf("failed to parse message at index %d: %w", i, err)
		}

		messages = append(messages, *message)
	}

	return messages, nil
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
