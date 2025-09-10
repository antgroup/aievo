package environment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/antgroup/aievo/schema"
	"github.com/thoas/go-funk"
)

// msg dispatch
func (e *Environment) dispatch(ctx context.Context, msg *schema.Message) error {
	e.Callback.HandleMessageInQueue(ctx, msg)
	if handler, exists := e.strategies[msg.Type]; exists {
		return handler(ctx, msg)
	}
	return nil
}

func (e *Environment) msgStrategy(ctx context.Context, msg *schema.Message) error {

	subscribers := e.Team.GetMsgSubMembers(msg)
	if msg.Receiver != "" && e.Agent(msg.Receiver) != nil {
		msg.AllReceiver = append(msg.AllReceiver, msg.Receiver)
	}
	if msg.IsMsg() {
		msg.AllReceiver = funk.UniqString(
			append(msg.AllReceiver, subscribers...))
	}
	return e.Memory.Save(ctx, *msg)
}

func (e *Environment) mngInfoStrategy(ctx context.Context, msg *schema.Message) error {
	if msg.MngInfo == nil {
		return nil
	}
	// only support 'Remove' currently
	if msg.MngInfo.Remove != nil {
		e.Team.RemoveMembers(msg.MngInfo.Remove)
	}
	if msg.MngInfo.Replace != nil {
		// Just clear the memory of the replaced agent.
		if len(msg.MngInfo.Replace) > 0 {
			_ = e.Memory.RemoveMessagesByAgents(ctx, msg.MngInfo.Replace)
			// 同时删除对应的动作历史记录
			_ = e.RemoveActionsByAgents(msg.MngInfo.Replace)
			msg.Receiver = msg.MngInfo.Replace[0]

			if msg.Receiver == "ALL" {
				msg.Receiver = e.GetTeamLeader().Name()
				allMembers := make([]string, 0, len(e.Team.members))
				for _, member := range e.Team.members {
					allMembers = append(allMembers, member.Name())
				}
				msg.AllReceiver = allMembers
				msg.Sender = "Watcher"
				msg.Content = msg.MngInfo.Content
				msg.Type = schema.MsgTypeMsg
				_ = e.Memory.Save(ctx, *msg)
			} else {
				if len(msg.MngInfo.Replace) == 1 {
					newInstruction := msg.MngInfo.Content
					// 获取需要更换的agent并更新其role
					targetAgent := e.Agent(msg.Receiver)
					if targetAgent != nil {
						// 将newInstruction添加到agent的role中
						currentRole := targetAgent.GetRole()
						targetAgent.SetRole(currentRole + "\nImportant Note: " + newInstruction)

						// 创建一条只有watcher能看到的消息记录
						watcherLogMsg := schema.Message{
							Type:        schema.MsgTypeMsg,
							Content:     fmt.Sprintf("Replace agent %s with instruction: %s", msg.Receiver, newInstruction),
							Sender:      "Watcher",
							Receiver:    "",
							AllReceiver: []string{"Watcher"}, // 只有watcher自己能看到
						}
						_ = e.Memory.Save(ctx, watcherLogMsg)
					}
				} else if len(msg.MngInfo.Replace) > 1 {
					// 有多个agent更新role
					agentGuidance := e.parseAgentGuidance(msg.MngInfo.Content)
					for _, agentName := range msg.MngInfo.Replace {
						if guidance, exists := agentGuidance[agentName]; exists {
							targetAgent := e.Agent(agentName)
							if targetAgent != nil {
								// 将对应的guidance添加到agent的role中
								currentRole := targetAgent.GetRole()
								targetAgent.SetRole(currentRole + "\nImportant Note: " + guidance)
							}
						}
					}

					// 创建一条只有watcher能看到的消息记录
					watcherLogMsg := schema.Message{
						Type:        schema.MsgTypeMsg,
						Content:     fmt.Sprintf("Replace multiple agents with guidance: %s", msg.MngInfo.Content),
						Sender:      "Watcher",
						Receiver:    "",
						AllReceiver: []string{"Watcher"}, // 只有watcher自己能看到
					}
					_ = e.Memory.Save(ctx, watcherLogMsg)
				}
			}
		}
	}
	// _ = e.Memory.Save(ctx, *msg)
	return nil
}

func (e *Environment) sopStrategy(ctx context.Context, msg *schema.Message) error {
	e.Sop = msg.Content
	e.Callback.HandleSOP(ctx, e.Sop)
	return nil
}

// parseAgentGuidance 解析JSON格式的agent指导字符串
// 例如: {"Accommodation Planner": "Verify physical addresses...", "Restaurant Planner": "SKIP ALL MEALS..."}
// 返回: map[agentName]guidance
func (e *Environment) parseAgentGuidance(content string) map[string]string {
	result := make(map[string]string)

	// 尝试解析JSON格式
	err := json.Unmarshal([]byte(content), &result)
	if err != nil {
		// 如果JSON解析失败，返回空map
		return make(map[string]string)
	}

	return result
}
