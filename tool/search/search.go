package search

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

const (
	_defaultTopK   = 6
	_defaultEngine = "google"
)

var searchEngines = map[string]Factory{
	"google": NewGoogleSearch,
	"bing":   NewBingSearch,
	"baidu":  NewBaiduSearch,
}

type Tool struct {
	TopK   int
	Engine string
	client *Client
}

var _ tool.Tool = &Tool{}

func New(opts ...Option) (*Tool, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}
	factory, ok := searchEngines[options.Engine]
	if !ok {
		factory = searchEngines[_defaultEngine]
	}
	if options.TopK <= 0 {
		options.TopK = _defaultTopK
	}
	return &Tool{
		TopK:   options.TopK,
		Engine: options.Engine,
		client: factory(options.ApiKey),
	}, nil
}

func (t *Tool) Name() string {
	return fmt.Sprintf("%s Search", strings.ToUpper(t.Engine))
}

func (t *Tool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return fmt.Sprintf(`A wrapper around %s Search.
Useful for when you need to answer questions about current events, 
the input must be json schema: %s`, strings.ToUpper(t.Engine), string(bytes)) + `
Example Input: {\"query\": \"machine learning, LLM, AI\"}`
}

func (t *Tool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"query": {
				Type:        tool.TypeString,
				Description: fmt.Sprintf("the query to search on %s, must be English", strings.ToUpper(t.Engine)),
			},
		},
		Required: []string{"query"},
	}
}

func (t *Tool) Strict() bool {
	return true
}

func (t *Tool) Call(_ context.Context, input string) (string, error) {
	var m map[string]interface{}

	err := json.Unmarshal([]byte(input), &m)
	if err != nil {
		return "json unmarshal error, please try agent", nil
	}

	if m["query"] == nil || m["query"].(string) == "" {
		return "query is required", nil
	}

	var ret string
	var searchErr error

	// Retry logic with exponential backoff
	maxRetries := 3
	baseDelay := 1 * time.Second
	for i := 0; i < maxRetries; i++ {
		ret, searchErr = t.client.Search(m["query"].(string), t.TopK)
		if searchErr == nil {
			return ret, nil
		}
		fmt.Printf("---Search Error (Attempt %d/%d)--- %v\n", i+1, maxRetries, searchErr)
		if i < maxRetries-1 {
			delay := baseDelay * time.Duration(math.Pow(2, float64(i)))
			fmt.Printf("Retrying in %v...\n", delay)
			time.Sleep(delay)
		}
	}

	fmt.Println("---Search Error after all retries---", searchErr)
	return "Query Search Engine Error, Please Try Again", nil
}
