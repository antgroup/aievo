package client

import (
	"github.com/mark3labs/mcp-go/client"
)

type TransportType string

const (
	TransportTypeSSE        TransportType = "SSE"
	TransportTypeStdio      TransportType = "Stdio"
	TransportTypeStreamHTTP TransportType = "StreamHTTP"
)

type ServerParam struct {
	// for stdio
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Cwd     string            `json:"cwd"`

	// for sse
	Url            string            `json:"url"`
	Headers        map[string]string `json:"headers"`
	Timeout        int               `json:"timeout"`
	SSEReadTimeout int               `json:"sseReadTimeout"`

	TransportType TransportType
}

type Client struct {
	name  string
	param *ServerParam
	// for sse client, init when use it
	c client.MCPClient
}
