package agent

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/utils/json"
	"github.com/goccy/go-graphviz"
)

var (
	_dotParse = regexp.MustCompile("(?s)```dot\n(.*?)\n```")
)

func NewSopAgent(opts ...Option) (schema.Agent, error) {
	opts = append(defaultSopOptions(), opts...)

	return NewBaseAgent(opts...)
}

func parseSopOutput(_ string, output *llm.Generation) ([]schema.StepAction, []schema.Message, error) {
	if output == nil {
		return nil, nil, errors.New("output is empty")
	}

	if len(output.ToolCalls) > 0 {
		return parseToolCalls(output.ToolCalls), nil, nil
	}
	content := strings.TrimSpace(output.Content)
	if content == "" {
		return nil, nil, errors.New("content is empty")
	}
	jsonContent := json.TrimJsonString(content)
	dotContent := extractDOTContent(jsonContent)
	action, err := parseAction(dotContent)
	if err == nil && action != nil {
		return []schema.StepAction{*action}, nil, nil
	}
	if err := validateDOT(dotContent); err != nil {
		return nil, nil, fmt.Errorf("parse sop to dot graph failed, err: %w", err)
	}
	message := createSOPMessage(jsonContent, dotContent)
	return nil, []schema.Message{message}, nil
}

func extractDOTContent(content string) string {
	submatch := _dotParse.FindAllStringSubmatch(content, -1)
	if len(submatch) > 0 {
		return strings.TrimSpace(submatch[0][1])
	}
	return content
}

func validateDOT(dotContent string) error {
	_, err := graphviz.ParseBytes([]byte(dotContent))
	return err
}

func createSOPMessage(jsonContent, dotContent string) schema.Message {
	return schema.Message{
		Type:    schema.MsgTypeSOP,
		Content: dotContent,
		Thought: jsonContent,
		Log:     jsonContent,
	}
}
