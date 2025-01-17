package qwen

import (
	"context"
	"github.com/antgroup/aievo/llm"
	"github.com/pkg/errors"
	goopenai "github.com/sashabaranov/go-openai"
)

type LLM struct {
	client         *goopenai.Client
	model          string
	ResponseFormat *goopenai.ChatCompletionResponseFormat
}

var (
	_             llm.LLM = (*LLM)(nil)
	_defaultModel         = "qwen-plus"
)

// newClient creates an instance of the internal client.
func newClient(opt *options) (*goopenai.Client, error) {

	if len(opt.token) == 0 {
		return nil, errors.New("missing the OpenAI API key, set it in the OPENAI_API_KEY environment variable")
	}

	config := goopenai.DefaultConfig(opt.token)
	if opt.apiType == goopenai.APITypeAzure {
		config = goopenai.DefaultAzureConfig(
			opt.token, opt.baseURL)
	}
	config.BaseURL = opt.baseURL
	config.OrgID = opt.organization

	if opt.httpClient != nil {
		config.HTTPClient = opt.httpClient
	}
	if opt.apiVersion != "" {
		config.APIVersion = opt.apiVersion
	}

	client := goopenai.NewClientWithConfig(config)

	return client, nil
}

func (L LLM) Generate(ctx context.Context, prompt string, options ...llm.GenerateOption) (*llm.Generation, error) {
	//TODO implement me
	panic("implement me")
}

func (L LLM) GenerateContent(ctx context.Context, messages []llm.Message, options ...llm.GenerateOption) (*llm.Generation, error) {
	//TODO implement me
	panic("implement me")
}
