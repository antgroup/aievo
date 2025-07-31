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
	// Filter out tools that are not relevant or not needed
	ignoredTools := map[string]struct{}{
		"firecrawl_scrape":             {},
		"firecrawl_generate_llmstxt":   {},
		"firecrawl_deep_research":      {},
		"firecrawl_extract":            {},
		"firecrawl_check_crawl_status": {},
		"firecrawl_crawl":              {},
		"firecrawl_map":                {},
		"browser_close":                {},
		"browser_resize":               {},
		"browser_console_messages":     {},
		"browser_handle_dialog":        {},
		"browser_evaluate":             {},
		"browser_file_upload":          {},
		"browser_install":              {},
		"browser_press_key":            {},
		"browser_network_requests":     {},
		"browser_take_screenshot":      {},
		"browser_drag":                 {},
		"browser_hover":                {},
		"browser_select_option":        {},
		"browser_tab_list":             {},
		"browser_tab_new":              {},
		"browser_tab_select":           {},
		"browser_tab_close":            {},
		"browser_wait_for":             {},
	}

	for _, t := range ts {
		if _, ok := ignoredTools[t.Name]; ok {
			continue
		}
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
	ret := string(marshal)
	if len(ret) > 2000 {
		ret = ret[:2000]
	}
	return ret, nil
}

func (t Tool) Schema() *tool.PropertiesSchema {
	return t.properties
}

func (t Tool) Strict() bool {
	return true
}
