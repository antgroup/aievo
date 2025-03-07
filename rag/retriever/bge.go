package retriever

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antgroup/aievo/utils/json"
	"github.com/antgroup/aievo/utils/request"
)

type BgeRetriever struct {
	providerUrl string
}

func NewBgeRetriever(opts ...Option) *BgeRetriever {
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	return &BgeRetriever{
		providerUrl: options.ProviderUrl,
	}
}

type BgeRequest struct {
	Text   string   `json:"text"`
	Source []string `json:"source"`
	Limit  int      `json:"limit"`
}

type BgeResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Data    []string `json:"data"`
}

func (r *BgeRetriever) Query(_ context.Context, text string, source []string, limit int) ([]string, error) {
	if text == "" {
		return nil, fmt.Errorf("text is empty")
	}
	if len(source) == 0 {
		return nil, fmt.Errorf("source is empty")
	}
	if limit == 0 {
		return nil, fmt.Errorf("limit must be no zero")
	}

	req := BgeRequest{
		Text:   text,
		Source: source,
		Limit:  limit,
	}

	marshal, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	var response BgeResponse

	err = request.Request(http.MethodPost, r.providerUrl, string(marshal), &response)
	if err != nil {
		return nil, fmt.Errorf("request bge failed: %w", err)
	}

	return response.Data, nil
}
