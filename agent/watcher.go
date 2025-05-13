package agent

import (
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/utils/json"
)

func NewWatcherAgent(opts ...Option) (schema.Agent, error) {
	opts = append(defaultWatcherOptions(), opts...)

	return NewBaseAgent(opts...)
}

func parseMngInfoOutput(_ string, output *llm.Generation) ([]schema.StepAction, []schema.Message, error) {
	if len(output.ToolCalls) > 0 {
		return parseToolCalls(output.ToolCalls), nil, nil
	}
	content := json.TrimJsonString(strings.TrimSpace(output.Content))
	action, err := parseAction(content)
	if err != nil {
		return nil, nil, err
	}
	if action != nil {
		return []schema.StepAction{*action}, nil, nil
	}
	message, err := parseMngInfoMessage(content)
	if err != nil {
		return nil, nil, err
	}
	return nil, []schema.Message{*message}, nil
}

func parseMngInfoMessage(content string) (*schema.Message, error) {
	finish := schema.Message{
		Type:    schema.MsgTypeCreative,
		MngInfo: &schema.MngInfo{},
	}
	if err := json.Unmarshal([]byte(content), finish.MngInfo); err != nil {
		return nil, err
	}
	return &finish, nil
}
