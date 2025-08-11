package schema

import (
	"context"
)

type Environment interface {
	// Produce produce msg
	Produce(ctx context.Context, msgs ...Message) error
	// Consume consume msg
	Consume(ctx context.Context) *Message

	// SOP task SOP
	SOP() string
	// GetTeam all team members
	GetTeam() []Agent
	// GetTeamLeader team Leader
	GetTeamLeader() Agent

	// LoadMemory get Agent's historical msg
	LoadMemory(ctx context.Context, receiver Agent) []Message

	GetSubscribeAgents(_ context.Context, subscribed Agent) []Agent

    WatchActionTaken(ctx context.Context, agentName string, steps []StepAction) string
}

// Memory is the interface for memory in chains.
type Memory interface {
	Load(ctx context.Context, filter func(index, consumption int, message Message) bool) []Message

	// LoadNext load next msg，filter check next msg，if not passed，then do not return
	LoadNext(ctx context.Context, filter func(message Message) bool) *Message

	Save(ctx context.Context, msg Message) error
	// Clear memory contents.
	Clear(ctx context.Context) error

	RemoveMessagesByAgents(ctx context.Context, agents []string) error
}

const (
	MsgTypeMsg      = "MSG"
	MsgTypeCreative = "CREATIVE"
	MsgTypeSOP      = "SOP"
	MsgTypeEnd      = "END"
)

const (
	MsgAllReceiver = "ALL"
)

type MngInfo struct {
	Create []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tools       []string `json:"tools"`
		Prompt      string   `json:"prompt"`
	} `json:"create"`
	Select []string `json:"select"`
	Remove []string `json:"remove"`
	Replace []string `json:"replace"`
}

type Subscribe struct {
	Subscribed Agent
	Subscriber Agent
	Condition  string
}
