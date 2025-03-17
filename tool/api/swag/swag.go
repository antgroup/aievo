package swag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	ErrUnSupportMethod  = errors.New("unsupport method")
	ErrUnSupportContent = errors.New("unsupport content type")
)

type API struct {
	OperationId string            `json:"operation_id"`
	Server      string            `json:"server"`
	Url         string            `json:"url"`
	Desc        string            `json:"desc"`
	Method      string            `json:"method"`
	ContentType string            `json:"content_type"`
	Query       map[string]string `json:"query"`
	Path        map[string]string `json:"path"`
	Body        map[string]string `json:"body"`
	Header      map[string]string `json:"header"`
	Params      map[string]string `json:"params"`
	Files       map[string]string `json:"files"`
	Requires    []string          `json:"requires"`
}

func Parse(_ context.Context, content []byte) ([]*API, error) {
	swagger, err := openapi3.NewLoader().LoadFromData(content)
	if err != nil {
		return nil, err
	}
	baseURL := swagger.Servers[0].URL
	apis := make([]*API, 0, len(swagger.Paths.Map()))

	for path, pathItem := range swagger.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			api := &API{
				OperationId: operation.OperationID,
				Server:      baseURL,
				Url:         baseURL + path,
				Desc:        operation.Summary,
				Method:      method,
				ContentType: "",
				Query:       make(map[string]string),
				Path:        make(map[string]string),
				Body:        make(map[string]string),
				Header:      make(map[string]string),
				Params:      make(map[string]string),
				Files:       make(map[string]string),
				Requires:    make([]string, 0, len(operation.Parameters)),
			}
			var contentType string
			if operation.RequestBody != nil {
				for ct, contents := range operation.RequestBody.Value.Content {
					contentType = ct
					for field, ref := range contents.Schema.Value.Properties {
						api.Body[field] = ref.Value.Description
						api.Params[field] = ref.Value.Description
					}
					api.Requires = append(api.Requires, contents.Schema.Value.Required...)
					break
				}
			}
			api.ContentType = contentType

			// fill in params
			for _, parameter := range operation.Parameters {
				api.Params[parameter.Value.Name] = parameter.Value.Description
				if parameter.Value.In == "query" {
					api.Query[parameter.Value.Name] = ""
				}
				if parameter.Value.In == "path" {
					api.Path[parameter.Value.Name] = ""
				}
				if parameter.Value.In == "body" {
					if parameter.Value.Schema.Value.Format == "binary" {
						api.Files[parameter.Value.Name] = ""
					} else {
						api.Body[parameter.Value.Name] = ""
					}
				}
				if parameter.Value.In == "header" {
					api.Header[parameter.Value.Name] = ""
				}
				if parameter.Value.Required {
					api.Requires = append(api.Requires, parameter.Value.Name)
				}
			}

			apis = append(apis, api)
		}
	}
	return apis, nil
}

func (api *API) Request(ctx context.Context, m map[string]string) (string, error) {
	err := api.ParseParam(ctx, m)
	if err != nil {
		return "", err
	}
	// begin to build request
	if strings.ToUpper(api.Method) == http.MethodGet {
		return api.GetRequest(ctx)
	}
	if strings.ToUpper(api.Method) == http.MethodPost {
		return api.PostRequest(ctx)
	}
	return "", ErrUnSupportMethod
}

func (api *API) ParseParam(ctx context.Context, m map[string]string) error {
	// according to input string, parse it into query body header

	misses := make([]string, 0, len(api.Requires))
	for _, require := range api.Requires {
		if m[require] == "" {
			misses = append(misses, require)
		}
	}
	if len(misses) > 0 {
		return errors.New("missing required parameters: " + strings.Join(misses, ","))
	}

	for key, v := range m {
		if _, ok := api.Query[key]; ok {
			api.Query[key] = v
			continue
		}
		if _, ok := api.Path[key]; ok {
			api.Url = strings.ReplaceAll(api.Url, "{"+key+"}", v)
			continue
		}

		if _, ok := api.Header[key]; ok {
			api.Header[key] = v
			continue
		}
		if _, ok := api.Body[key]; ok {
			api.Body[key] = v
			continue
		}
		if _, ok := api.Files[key]; ok {
			api.Files[key] = v
			continue
		}
	}
	return nil
}

func (api *API) GetRequest(ctx context.Context) (string, error) {
	req, err := http.NewRequest(http.MethodGet, api.Url, nil)
	if err != nil {
		return "", err
	}
	api.addParametersToRequest(req)
	return api.do(ctx, req)
}

func (api *API) PostRequest(ctx context.Context) (string, error) {
	var reqBody *bytes.Reader
	contentType := api.ContentType
	switch api.ContentType {
	case "application/json":
		marshal, _ := json.Marshal(api.Body)
		reqBody = bytes.NewReader(marshal)
	case "application/x-www-form-urlencoded":
		data := url.Values{}
		for key, v := range api.Body {
			data.Set(key, v)
		}
		reqBody = bytes.NewReader([]byte(data.Encode()))
	case "multipart/form-data":
		// because llm wont give file binary, so dont use this type
		return "", ErrUnSupportContent
	case "":
		// do nothing
	default:
		return "", ErrUnSupportContent
	}
	req, err := http.NewRequest(http.MethodPost, api.Url, reqBody)
	if err != nil {
		return "", err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	api.addParametersToRequest(req)
	return api.do(ctx, req)
}

func (api *API) do(_ context.Context, req *http.Request) (string, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(all), nil
}

func (api *API) addParametersToRequest(req *http.Request) {
	q := req.URL.Query()
	for key, v := range api.Query {
		q.Add(key, v)
	}
	req.URL.RawQuery = q.Encode()
	for key, v := range api.Header {
		req.Header.Add(key, v)
	}
	return
}
