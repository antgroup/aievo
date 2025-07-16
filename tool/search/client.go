package search

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	_baseUrl = "https://serpapi.com"
)

var ErrorNoSearchParamsHandler = errors.New("no search params handler")
var ErrorNoSearchResultHandler = errors.New("no search result handler")
var ErrorNoRequiredField = errors.New("no required field")

type Client struct {
	Engine              string
	ApiKey              string
	HttpSearch          *http.Client
	SearchParamsHandler SerpSearchParamsHandler
	SearchResultHandler SerpSearchResultHandler
}

func NewSearch(engine string,
	apiKey string,
	paramsHandler SerpSearchParamsHandler,
	resultHandler SerpSearchResultHandler) *Client {
	// Create the http search
	httpSearch := &http.Client{
		Timeout: time.Second * 120, // Set a timeout of 120 seconds
	}
	return &Client{
		Engine:              engine,
		ApiKey:              apiKey,
		HttpSearch:          httpSearch,
		SearchParamsHandler: paramsHandler,
		SearchResultHandler: resultHandler,
	}
}

// Search get content from search engine
func (client *Client) Search(input string, topK int) (string, error) {
	if client.SearchParamsHandler == nil {
		return "", ErrorNoSearchParamsHandler
	}
	if client.SearchResultHandler == nil {
		return "", ErrorNoSearchResultHandler
	}
	datas := make([]interface{}, 0)
	pageIndex := 1
	pageSize := topK
	for len(datas) < topK {
		params, _ := client.SearchParamsHandler.Handle(input, pageIndex, pageSize)
		rsp, err := client.execute(params, "/search", "json")
		if err != nil {
			return "", err
		}
		ret, err := client.decodeJSON(rsp.Body)
		if err != nil {
			return "", err
		}
		if ret[client.SearchResultHandler.GetRequiredField()] == nil {
			return "", ErrorNoRequiredField
		}
		datas = append(datas, ret[client.SearchResultHandler.GetRequiredField()].([]interface{})...)
		pageIndex++
	}
	datas = datas[:topK]
	jsn, _ := json.Marshal(datas)
	return client.SearchResultHandler.Handle(string(jsn))
}

// execute HTTP get request and returns http response
func (client *Client) execute(params map[string]string, path string, output string) (*http.Response, error) {
	query := url.Values{}
	for k, v := range params {
		query.Add(k, v)
	}

	// api_key
	if len(client.ApiKey) != 0 {
		query.Add("api_key", client.ApiKey)
	}

	// engine
	if len(query.Get("engine")) == 0 {
		query.Set("engine", client.Engine)
	}

	// source programming language
	query.Add("source", "go")

	// set output
	query.Add("output", output)

	endpoint := _baseUrl + path + "?" + query.Encode()
	rsp, err := client.HttpSearch.Get(endpoint)
	if err != nil {
		return nil, err
	}
	return rsp, nil
}

// decodeJson response
func (client *Client) decodeJSON(body io.ReadCloser) (SerpSearchResult, error) {
	// Decode JSON from response body
	decoder := json.NewDecoder(body)

	// Response data
	var rsp SerpSearchResult
	err := decoder.Decode(&rsp)
	if err != nil {
		return nil, errors.New("fail to decode")
	}

	// check error message
	errorMessage, ok := rsp["error"].(string)
	if ok {
		return nil, errors.New(errorMessage)
	}
	return rsp, nil
}
