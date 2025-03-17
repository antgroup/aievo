package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/tool/api/swag"
)

// Tool defines a tool implementation for the DuckDuckGo Search.
type Tool struct {
	api    *swag.API
	suffix string
	desc   string
}

var _ tool.Tool = Tool{}

// New initializes a new api client from parse schema
func New(schema string, opts ...Option) ([]tool.Tool, error) {
	apis, err := swag.Parse(context.Background(), []byte(schema))
	if err != nil {
		return nil, err
	}
	tools := make([]tool.Tool, 0, len(apis))
	for _, api := range apis {
		desc := api.Desc
		t := &Tool{
			api: api,
		}
		for _, opt := range opts {
			opt(t)
		}
		if len(api.Params) != 0 {
			marshal, _ := json.Marshal(api.Params)
			desc += ", the field type is string, the input must be json format like " + string(marshal)
		}
		t.desc = desc

		tools = append(tools, *t)
	}

	return tools, nil
}

// Name returns a name for the tool.
func (t Tool) Name() string {
	return t.api.OperationId
}

// Description returns a description for the tool.
func (t Tool) Description() string {
	return t.desc
}

// Call performs the search and return the result.
func (t Tool) Call(ctx context.Context, input string) (string, error) {

	m := map[string]interface{}{}
	params := make(map[string]string)
	err := json.Unmarshal([]byte(input), &m)
	for k, v := range m {
		// change v to string
		switch v.(type) {
		case string:
			params[k] = v.(string)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			params[k] = fmt.Sprintf("%d", v)
		case float32, float64:
			// check int
			if math.Mod(v.(float64), 1) == 0 {
				params[k] = strconv.FormatInt(int64(v.(float64)), 10)
			} else {
				params[k] = strconv.FormatFloat(v.(float64), 'f', -1, 64)
			}
		case bool:
			params[k] = fmt.Sprintf("%t", v)
		default:
			// skip for other type
			return fmt.Sprintf("Unsupported type for key %s: %T, please give string type\n", k, v), nil
		}
	}
	if err != nil {
		return "execute api failed，parse json error: " + err.Error(), nil
	}

	result, err := t.api.Request(ctx, params)

	if err != nil {
		result = "execute api failed，err: " + err.Error()
		err = nil
	}

	return result + "\n" + t.suffix, nil
}

func (t Tool) Schema() *tool.PropertiesSchema {
	return nil
}

func (t Tool) Strict() bool {
	return true
}
