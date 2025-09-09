package environment

import (
	"context"

	"github.com/antgroup/aievo/callback"
	"github.com/antgroup/aievo/memory"
	"github.com/antgroup/aievo/schema"
)

// ActionRecord 记录agent的动作历史
type ActionRecord struct {
	AgentName string `json:"agent_name"`
	Action    string `json:"action"`
	Input     string `json:"input"`
	Output    string `json:"output"`
	Timestamp string `json:"timestamp"`
}

type Environment struct {
	Team                  *Team
	SopExpert             schema.Agent
	Planner               schema.Agent
	Watcher               schema.Agent
	WatchCondition        func(message schema.Message, memory schema.Memory) bool
	WatcherInterval       int // 每几轮对话后触发一次watcher
	WatcherActionInterval int // 每几轮动作后触发一次watcher
	WatchChan             chan schema.Message
	WatchChanDone         chan struct{}
	Memory                schema.Memory
	ActionHistory         []ActionRecord // 所有agent的动作历史记录
	Callback              callback.Handler
	MaxTurn               int
	MaxToken              int
	Sop                   string

	strategies map[string]func(context.Context, *schema.Message) error

	turn  int
	token int
}

func NewEnv() *Environment {
	e := &Environment{
		Team:   NewTeam(),
		Memory: memory.NewBufferMemory(),
	}
	e.strategies = map[string]func(ctx context.Context, msg *schema.Message) error{
		schema.MsgTypeMsg:      e.msgStrategy,
		schema.MsgTypeEnd:      e.msgStrategy,
		schema.MsgTypeSOP:      e.sopStrategy,
		schema.MsgTypeCreative: e.mngInfoStrategy,
	}
	return e
}
