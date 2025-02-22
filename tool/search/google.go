package search

import (
	"encoding/json"
	"errors"
	"strconv"
)

var ErrorEncodeGoogleResp = errors.New("error encoding Google search result")

// NewGoogleSearch creates search for Google
func NewGoogleSearch(apiKey string) *Client {
	return NewSearch(
		"google",
		apiKey,
		&GoogleSearchParamsHandler{},
		&GoogleSearchResultHandler{
			RequiredField: "organic_results",
		})
}

type GoogleSearchParamsHandler struct {
}

func (h *GoogleSearchParamsHandler) Handle(input string, pageIndex, pageSize int) (map[string]string, error) {
	return map[string]string{
		"q":     input,
		"start": strconv.Itoa((pageIndex - 1) * pageSize),
		"count": strconv.Itoa(pageSize),
	}, nil
}

type GoogleSearchResultHandler struct {
	RequiredField string
}

func (h *GoogleSearchResultHandler) GetRequiredField() string {
	return h.RequiredField
}

func (h *GoogleSearchResultHandler) Handle(result string) (string, error) {
	var googleDatas []GoogleData
	err := json.Unmarshal([]byte(result), &googleDatas)
	if err != nil {
		return "", ErrorEncodeGoogleResp
	}
	var output string
	for _, data := range googleDatas {
		output += Title + data.Title + "\n"
		output += Snippet + data.Snippet + "\n"
		output += Link + data.Link + "\n\n"
	}
	return output, nil
}
