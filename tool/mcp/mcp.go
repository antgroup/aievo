package mcp

import (
	"context"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/tool/mcp/client"
	"github.com/antgroup/aievo/utils/json"
	"github.com/mark3labs/mcp-go/mcp"
)

// Tool defines a tool implementation for the MCP tool proxy Search.
type Tool struct {
	client     *client.Client
	name       string
	desc       string
	properties *tool.PropertiesSchema
}

var _ tool.Tool = Tool{}

// New initializes mcp clients from parse schema
func New(schema string) ([]tool.Tool, error) {
	ctx := context.Background()
	mcpServers, err := ParseMcpServers(ctx, schema)
	if err != nil {
		return nil, err
	}
	tools := make([]tool.Tool, 0, len(mcpServers))
	for name, param := range mcpServers {
		c, err := client.New(ctx, name, param)
		if err != nil {
			return nil, err
		}
		ts, err := c.ListTools(ctx)
		if err != nil {
			return nil, err
		}
		tools = append(tools, convertMCPTool2Tool(c, ts)...)
	}
	return tools, nil
}

func convertMCPTool2Tool(c *client.Client, ts []mcp.Tool) []tool.Tool {
	tools := make([]tool.Tool, 0, len(ts))

	for _, t := range ts {
		marshal, _ := json.Marshal(t.InputSchema)
		ct := &Tool{
			client:     c,
			name:       t.Name,
			desc:       t.Description,
			properties: &tool.PropertiesSchema{},
		}
		_ = json.Unmarshal(marshal, ct.properties)
		tools = append(tools, ct)
	}
	return tools
}

// Name returns a name for the tool.
func (t Tool) Name() string {
	return t.name
}

// Description returns a description for the tool.
func (t Tool) Description() string {
	desc := t.desc
	if t.properties != nil {
		marshal, _ := json.Marshal(t.properties)
		desc += "\nthis is input schema for this tool:\n" + string(marshal)
	}
	return desc

}

// Call performs the search and return the result.
func (t Tool) Call(ctx context.Context, input string) (string, error) {
	input = json.TrimJsonString(input)
	result, err := t.client.CallTool(ctx, t.name, input)
	if err != nil {
		return "failed to call tool, err: " + err.Error(), nil
	}
	marshal, _ := json.Marshal(result)
	return string(marshal), nil
}

func (t Tool) Schema() *tool.PropertiesSchema {
	return t.properties
}

func (t Tool) Strict() bool {
	return true
}
