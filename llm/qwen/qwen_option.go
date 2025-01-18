package qwen

import (
	"github.com/antgroup/aievo/llm"
	goopenai "github.com/sashabaranov/go-openai"
	"net/http"
)

type options struct {
	token           string
	model           string
	stream          bool
	messages        []llm.Message
	temperature     float32
	topP            float32
	presencePenalty float32
	maxTokens       int
	nu              int
	seed            int
	stop            []string
	enableSearch    bool
	baseURL         string
	httpClient      *http.Client
	responseFormat  *goopenai.ChatCompletionResponseFormat
}

// Option is a functional option for the OpenAI client.
type Option func(*options)

// WithToken passes the OpenAI API token to the client. If not set, the token
// is read from the OPENAI_API_KEY environment variable.
func WithToken(token string) Option {
	return func(opts *options) {
		opts.token = token
	}
}

// WithModel passes the OpenAI model to the client. If not set, the model
// is read from the OPENAI_MODEL environment variable.
// Required when ApiType is Azure.
func WithModel(model string) Option {
	return func(opts *options) {
		opts.model = model
	}
}

// WithBaseURL passes the OpenAI base url to the client. If not set, the base url
// is read from the OPENAI_BASE_URL environment variable. If still not set in ENV
// VAR OPENAI_BASE_URL, then the default value is https://api.openai.com/v1 is used.
func WithBaseURL(baseURL string) Option {
	return func(opts *options) {
		opts.baseURL = baseURL
	}
}

// WithTopP passes the OpenAI top_p to the client. If not set, the default value is 1.0.
func WithTopP(topP float32) Option {
	return func(opts *options) {
		opts.topP = topP
	}
}

// WithPresencePenalty passes the OpenAI presence_penalty to the client. If not set,
// the default value is 0.0.
func WithPresencePenalty(presencePenalty float32) Option {
	return func(opts *options) {
		opts.presencePenalty = presencePenalty
	}
}

// WithMaxTokens passes the OpenAI max_tokens to the client.
func WithMaxTokens(maxTokens int) Option {
	return func(opts *options) {
		opts.maxTokens = maxTokens
	}
}

// WithNu passes the QWen nu to the client.
func WithNu(nu int) Option {
	return func(opts *options) {
		opts.nu = nu
	}
}

// WithSeed passes the QWen seed to the client.
func WithSeed(seed int) Option {
	return func(opts *options) {
		opts.seed = seed
	}
}

// WithStream passes the QWen stream to the client.
func WithStream(stream bool) Option {
	return func(opts *options) {
		opts.stream = stream
	}
}

// WithStop passes the QWen stop to the client.
func WithStop(stop []string) Option {
	return func(opts *options) {
		opts.stop = stop
	}
}

// WithEnableSearch passes the QWen search in internet to the client.
func WithEnableSearch(enableSearch bool) Option {
	return func(opts *options) {
		opts.enableSearch = enableSearch
	}
}

// WithBaseUrl is deprecated, use WithBaseURL instead.
func WithBaseUrl(baseURL string) Option {
	return func(opts *options) {
		opts.baseURL = baseURL
	}
}

// WithHttpClient passes the QWen http client to the client.
func WithHttpClient(httpClient *http.Client) Option {
	return func(opts *options) {
		opts.httpClient = httpClient
	}
}

// WithResponseFormat passes the QWen response format to the client.
func WithResponseFormat(responseFormat *goopenai.ChatCompletionResponseFormat) Option {
	return func(opts *options) {
		opts.responseFormat = responseFormat
	}
}

// WithTemperature passes the QWen temperature to the client.
func WithTemperature(temperature float32) Option {
	return func(opts *options) {
		opts.temperature = temperature
	}
}

func WithMessages(messages []Message) Option {
	return func(opts *options) {
		opts.messages = messages
	}
}

func WithMessage(message Message) Option {
	return func(opts *options) {
		opts.messages = append(opts.messages, message)
	}
}
