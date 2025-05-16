package mcp

import (
	"context"
	"errors"

	"github.com/antgroup/aievo/tool/mcp/client"
	"github.com/antgroup/aievo/utils/json"
)

type MCPServers struct {
	MCPServers map[string]*client.ServerParam `json:"mcpServers"`
}

func ParseMcpServers(ctx context.Context, schema string) (map[string]*client.ServerParam, error) {
	mcpServers := &MCPServers{}
	err := json.Unmarshal([]byte(schema), &mcpServers)
	if err != nil {
		return nil, err
	}
	// analyse transport type
	for server, param := range mcpServers.MCPServers {
		if param.Command == "" && param.Url != "" {
			param.TransportType = client.TransportTypeSSE
			continue
		}
		if param.Command != "" && param.Url == "" {
			param.TransportType = client.TransportTypeStdio
			continue
		}
		return nil, errors.New("cannot analyse mcp transport type for " + server)
	}
	return mcpServers.MCPServers, nil
}
