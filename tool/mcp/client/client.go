package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func New(ctx context.Context, name string, param *ServerParam) (*Client, error) {
	c := &Client{
		name:  name,
		param: param,
	}
	var mcpClient client.MCPClient
	var err error
	switch param.TransportType {
	case TransportTypeSSE:
		mcpClient, err = c.initSSEClient(ctx)
	case TransportTypeStdio:
		mcpClient, err = c.initStdioClient(ctx)
	default:
		return nil, fmt.Errorf("unsupported mcp client transport type: %s", param.TransportType)
	}
	if err != nil {
		return nil, err
	}
	c.c = mcpClient
	go c.refresh()
	return c, nil
}

func (c *Client) initStdioClient(ctx context.Context) (client.MCPClient, error) {
	envs := make([]string, 0, len(c.param.Env))
	for k, v := range c.param.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	mc, err := client.NewStdioMCPClient(
		c.param.Command, envs, c.param.Args...)
	if err != nil {
		log.Printf("failed to initialize stdio client: %v", err)
		return nil, err
	}
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{
		Name:    "aievo-client",
		Version: "1.0.0",
	}
	_, err = mc.Initialize(ctx, request)
	if err != nil {
		log.Printf("error initializing mcp client: %v", err)
		return nil, err
	}
	return mc, nil
}

func (c *Client) initSSEClient(ctx context.Context) (client.MCPClient, error) {
	path, _ := url.JoinPath(c.param.Url, "/sse")
	mc, err := client.NewSSEMCPClient(path)
	if err != nil {
		log.Printf("failed to initialize SSE client: %v", err)
		return nil, err
	}

	if err = mc.Start(ctx); err != nil {
		log.Printf("failed to start SSE client: %v", err)
		return nil, err
	}

	// Initialize
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "aievo-client",
		Version: "1.0.0",
	}

	_, err = mc.Initialize(ctx, initRequest)
	if err != nil {
		log.Printf("failed to initialize mcp client: %v", err)
		return nil, err
	}
	return mc, nil
}

func (c *Client) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	var err error
	toolsRequest := mcp.ListToolsRequest{}
	result, err := c.c.ListTools(ctx, toolsRequest)
	if err != nil {
		log.Printf("failed to list tools: %v", err)
		return nil, err
	}
	return result.Tools, nil
}

func (c *Client) CallTool(ctx context.Context, name, input string) (*mcp.CallToolResult, error) {
	var err error
	request := mcp.CallToolRequest{}
	param := make(map[string]any)
	err = json.Unmarshal([]byte(input), &param)
	if err != nil {
		log.Printf("failed to unmarshal input: %v", err)
		return nil, err
	}
	request.Params.Name = name
	request.Params.Arguments = param

	result, err := c.c.CallTool(ctx, request)
	if err != nil {
		log.Printf("failed to call tool: %v", err)
		return nil, err
	}

	return result, nil

}

func (c *Client) refresh() {
	tick := time.Tick(time.Second * 15)
	for {
		<-tick
		ctx := context.Background()
		err := c.c.Ping(ctx)
		if err == nil {
			continue
		}
		log.Printf("failed to ping server: %v, refresh client", err)
		var mcpClient client.MCPClient
		switch c.param.TransportType {
		case TransportTypeSSE:
			mcpClient, err = c.initSSEClient(ctx)
		case TransportTypeStdio:
			mcpClient, err = c.initStdioClient(ctx)
		}
		if err != nil {
			log.Printf("failed to initialize mcp client: %v, try again after 15s", err)
			continue
		}
		oldClient := c.c
		c.c = mcpClient
		oldClient.Close()
	}
}

func (c *Client) Close() error {
	if c.c != nil {
		return c.c.Close()
	}
	return nil
}
