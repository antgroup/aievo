package text

import (
	"context"
	"fmt"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
	"strings"
)

var textDetector = map[string]Factory{
	"sensitivity-detection": NewSensitivityDetector,
}

const _defaultMode = "sensitivity-detection"

type Factory func(sensitiveWords []string, content, outputModel string) Processor

type Processor interface {
	Process() (string, error)
}

type Tool struct {
	Mode      string
	Processor Processor
}

func New(opts ...Option) (*Tool, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}
	processor, ok := textDetector[options.ProcessMode]
	if !ok {
		processor = textDetector[_defaultMode]
	}
	return &Tool{
		Mode:      options.ProcessMode,
		Processor: processor(options.Keywords, options.Content, options.ProcessMode),
	}, nil
}

var _ tool.Tool = &Tool{}

func (t Tool) Name() string {
	return fmt.Sprintf("%s Text Process", strings.ToUpper(t.Mode))
}

func (t Tool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return fmt.Sprintf(`A wrapper tool for %s Text Process.
Useful for when you need to process LLM's output text, 
the input must be json schema: %s`, strings.ToUpper(t.Mode), string(bytes)) + `
Example Input: {\"keywords\": [\"sensitive-words1\", \"sensitive-words2\"]}`
}

func (t Tool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"keywords": {
				Type:        tool.TypeArr,
				Description: fmt.Sprintf("the type of input keywords on %s must be string array", strings.ToUpper(t.Mode)),
			},
			"content": {
				Type:        tool.TypeString,
				Description: fmt.Sprintf("the type of input content on %s must be string array", strings.ToUpper(t.Mode)),
			},
		},
		Required: []string{"keywords"},
	}
}

func (t Tool) Strict() bool {
	return true
}

func (t Tool) Call(ctx context.Context, input string) (string, error) {
	var m map[string]interface{}

	err := json.Unmarshal([]byte(input), &m)
	if err != nil {
		return "json unmarshal error, please try agent", nil
	}

	if m["keywords"] == nil {
		return "keywords are required", nil
	}

	if m["content"] == nil {
		return "content are required", nil
	}

	ret, err := t.Processor.Process()
	if err != nil {
		return "Process Content Error, Please Try Again", nil
	}
	return ret, nil
}
