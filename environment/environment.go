package environment

import (
	"context"
	// "encoding/json"
	"fmt"
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
	"github.com/thoas/go-funk"
)

func (e *Environment) Produce(ctx context.Context, msgs ...schema.Message) error {
	for _, msg := range msgs {
		msg.Type = strings.ToUpper(msg.Type)
		e.token += msg.Token
		err := e.dispatch(ctx, &msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Consume When reach max token or max turn, consume return nil
// Consume will return next message unhandled,
// when next message is same receiver, it will be return instead of next message
func (e *Environment) Consume(ctx context.Context) *schema.Message {
	if (e.MaxTurn > 0 && e.turn > e.MaxTurn) ||
		(e.MaxToken > 0 && e.token > e.MaxToken) {
		return nil
	}
	e.turn++
	// 合并相同receiver的消息
	msg := e.Memory.LoadNext(ctx, nil)
	for {
		if e.Callback != nil {
			e.Callback.HandleMessageOutQueue(ctx, msg)
		}
		tmp := e.Memory.LoadNext(ctx, func(message schema.Message) bool {
			return message.Receiver == msg.Receiver
		})
		if tmp == nil {
			break
		}
		msg = tmp
	}
	return msg
}

func (e *Environment) LoadMemory(ctx context.Context, receiver schema.Agent) []schema.Message {
	// 按照当前消费位点，返回消息
	if receiver == nil || receiver == e.Watcher || receiver == e.SopExpert ||
		receiver == e.Planner {
		return e.Memory.Load(ctx, nil)
	}
	return e.Memory.Load(ctx, func(index, consumption int, message schema.Message) bool {
		if index <= consumption && (strings.EqualFold(message.Sender, receiver.Name()) ||
			funk.ContainsString(message.AllReceiver, receiver.Name())) {
			return true
		}
		return false
	})
}

func (e *Environment) Agent(name string) schema.Agent {
	return e.Team.Member(name)
}

func (e *Environment) GetSubscribeAgents(ctx context.Context,
	subscribed schema.Agent) []schema.Agent {
	if e.SopExpert == subscribed ||
		e.Planner == subscribed ||
		e.Watcher == subscribed {
		return e.GetTeam()
	}
	return e.Team.GetSubMembers(ctx, subscribed)
}

func (e *Environment) SOP() string {
	return e.Sop
}

func (e *Environment) GetTeam() []schema.Agent {
	return e.Team.members
}

func (e *Environment) GetTeamLeader() schema.Agent {
	return e.Team.Leader
}

// WatchActionTaken 调用watcher观察action行为
func (e *Environment) WatchActionTaken(ctx context.Context, agentName string, steps []schema.StepAction) string {
	// 1. 检查 Watcher 是否已准备好接收通知
	if e.WatchChan == nil {
		return "null"
	}

	// // 2. 将传入的 steps 历史序列化为 JSON
	// historyBytes, err := json.Marshal(steps)
	// if err != nil {
	// 	fmt.Printf("Error marshalling action history for agent %s: %v\n", agentName, err)
	// 	return "null"
	// }
    var nilmessage []schema.Message
    actionHistory := schema.ConvertConstructScratchPad("", agentName, nilmessage, steps)

	// 3. 创建系统消息，内容包含 agent 名称和其完整的 action 历史
	triggerMsg := schema.Message{
		Type:    "MSG",
		Content: fmt.Sprintf("Action history:\n%s", actionHistory),
		Sender:  agentName,
	}

	// 4. 发送消息并等待 Watcher 处理完毕
	// e.WatchChan <- triggerMsg
	// <-e.WatchChanDone

	historyMessages := e.LoadMemory(ctx, e.Watcher)
	opts := []llm.GenerateOption{
		llm.WithTemperature(0.6),
		llm.WithTopP(0.95),
	}
	generation, err := e.Watcher.Run(ctx, append(historyMessages, triggerMsg), opts...)
	if err != nil {
		fmt.Printf("Error running watcher for agent %s: %v\n", agentName, err)
		return "null"
	}
	msg := generation.Messages[0]
	if msg.MngInfo == nil {
		return "null"
	}
	// 如果replace不是空列表，则返回第一个元素
	if len(msg.MngInfo.Replace) > 0 {
		return msg.MngInfo.Replace[0]
	}
	return "null"
}
