package mcp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	schema := `{
  "mcpServers": {
    "sqlite": {
      "command": "/Users/tyloafer/.local/bin/uvx",
      "args": ["mcp-server-sqlite", "--db-path", "/Users/tyloafer/WorkPlace/ali/python-sdk/examples/clients/simple-chatbot/mcp_simple_chatbot/test.db"]
    }
  }
}`
	tools, err := New(schema)
	assert.Nil(t, err)
	assert.NotNil(t, tools)
	fmt.Println(tools)
}
