package qwen

import (
	goopenai "github.com/sashabaranov/go-openai"
	"net/http"
)

type options struct {
	token string
	model string

	stream       bool
	messages     []string
	baseURL      string
	organization string
	apiType      goopenai.APIType
	httpClient   *http.Client

	responseFormat *goopenai.ChatCompletionResponseFormat

	// required when APIType is APITypeAzure or APITypeAzureAD
	apiVersion string
}

// Option is a functional option for the OpenAI client.
type Option func(*options)
