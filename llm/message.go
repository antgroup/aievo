package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// MessageType is the type of chat message.
type MessageType string

// ErrUnexpectedMessageType is returned when a chat message is of an
// unexpected type.
var ErrUnexpectedMessageType = errors.New("unexpected message type")

const (
	// MessageTypeUser is a message sent by a human.
	MessageTypeUser MessageType = "user"
	// MessageTypeSystem is a message sent by the system.
	MessageTypeSystem MessageType = "system"
	// MessageTypeAssistant is a message sent by the assistant.
	MessageTypeAssistant MessageType = "assistant"
	// MessageTypeTool is a message sent by a tool.
	MessageTypeTool MessageType = "tool"
)

// Message is a message sent by an assistant.
type Message struct {
	Role       MessageType
	Name       string     `json:"name,omitempty"`
	ToolCallId string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	// Content is the content of the message.
	Content string `json:"content,omitempty"`
}

func NewUserMessage(name, content string) Message {
	return Message{
		Role:    MessageTypeUser,
		Name:    name,
		Content: content,
	}
}

func NewSystemMessage(name, content string) Message {
	return Message{
		Role:    MessageTypeSystem,
		Name:    name,
		Content: content,
	}
}

func NewAssistantMessage(name, content string, toolCalls []ToolCall) Message {
	return Message{
		Role:      MessageTypeAssistant,
		Name:      name,
		Content:   content,
		ToolCalls: toolCalls,
	}
}

func NewToolMessage(id, content string) Message {
	return Message{
		Role:       MessageTypeTool,
		ToolCallId: id,
		Content:    content,
	}
}

// ToolCall is a call to a tool (as requested by the model) that should be executed.
type ToolCall struct {
	// ID is the unique identifier of the tool call.
	ID string `json:"id"`
	// Type is the type of the tool call. Typically, this would be "function".
	Type string `json:"type"`
	// Function is the function call to be executed.
	Function *FunctionCall `json:"function,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// GetBufferString gets the buffer string of messages.
func GetBufferString(messages []Message) (string, error) {
	result := make([]string, 0, len(messages))
	for _, m := range messages {
		msg := fmt.Sprintf("%s: %s", m.Role, m.Content)
		if m.Role == MessageTypeAssistant && m.ToolCalls != nil {
			j, err := json.Marshal(m.ToolCalls)
			if err != nil {
				return "", err
			}
			msg = fmt.Sprintf("%s %s", msg, string(j))
		}
		result = append(result, msg)
	}
	return strings.Join(result, "\n"), nil
}
