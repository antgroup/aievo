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
	// bytes, _ := json.Marshal(t.Schema())
	return fmt.Sprintf(`A wrapper around %s Search.
Useful for when you need to answer questions about current events, 
the input must be json schema: {"query": "The search query string"}`, strings.ToUpper(t.Engine)) + `
Separate different queries with commas.
Example Input: {"query": "current US president, capital of Canada"}`
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

	queryString := m["query"].(string)

	// Split queries by comma and trim spaces
	queries := strings.Split(queryString, ", ")
	var trimmedQueries []string
	for _, query := range queries {
		trimmed := strings.TrimSpace(query)
		if trimmed != "" {
			trimmedQueries = append(trimmedQueries, trimmed)
		}
	}

	if len(trimmedQueries) == 0 {
		return "query is required", nil
	}

	var combinedResults []string

	// Search each query separately
	for i, query := range trimmedQueries {
		var ret string
		var searchErr error

		// Retry logic with exponential backoff for each query
		maxRetries := 3
		baseDelay := 1 * time.Second
		for j := 0; j < maxRetries; j++ {
			ret, searchErr = t.client.Search(query, t.TopK)
			if searchErr == nil {
				break
			}
			fmt.Printf("---Search Error for query %d (Attempt %d/%d)--- %v\n", i+1, j+1, maxRetries, searchErr)
			if j < maxRetries-1 {
				delay := baseDelay * time.Duration(math.Pow(2, float64(j)))
				fmt.Printf("Retrying in %v...\n", delay)
				time.Sleep(delay)
			}
		}

		if searchErr != nil {
			fmt.Printf("---Search Error for query %d after all retries--- %v\n", i+1, searchErr)
			combinedResults = append(combinedResults, fmt.Sprintf("Search result of query %d: Query Search Engine Error, Please Try Again\n", i+1))
		} else {
			combinedResults = append(combinedResults, fmt.Sprintf("Search result of query %d:\n %s", i+1, ret))
		}
	}

	// Combine all results
	return strings.Join(combinedResults, "\n"), nil
}
