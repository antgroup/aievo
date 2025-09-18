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
				// 处理Content字段类型转换
				switch v := msg.MngInfo.Content.(type) {
				case string:
					msg.Content = v
				default:
					if contentBytes, err := json.Marshal(v); err == nil {
						msg.Content = string(contentBytes)
					} else {
						msg.Content = fmt.Sprintf("%v", v)
					}
				}
				msg.Type = schema.MsgTypeMsg
				_ = e.Memory.Save(ctx, *msg)
			} else {
				if len(msg.MngInfo.Replace) == 1 {
					var newInstruction string
					// 处理Content字段，支持字符串和其他类型
					switch v := msg.MngInfo.Content.(type) {
					case string:
						newInstruction = v
					default:
						// 如果不是字符串，将其序列化为JSON字符串
						if contentBytes, err := json.Marshal(v); err == nil {
							newInstruction = string(contentBytes)
						} else {
							newInstruction = fmt.Sprintf("%v", v)
						}
					}

					// 获取需要更换的agent并更新其role
					targetAgent := e.Agent(msg.Receiver)
					if targetAgent != nil {
						// 将newInstruction添加到agent的role中
						currentRole := targetAgent.GetRole()
						targetAgent.SetRole(currentRole + "\n**Important Note:**\n" + newInstruction)

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
					var contentStr string
					switch v := msg.MngInfo.Content.(type) {
					case string:
						contentStr = v
					default:
						if contentBytes, err := json.Marshal(v); err == nil {
							contentStr = string(contentBytes)
						} else {
							contentStr = fmt.Sprintf("%v", v)
						}
					}

					watcherLogMsg := schema.Message{
						Type:        schema.MsgTypeMsg,
						Content:     fmt.Sprintf("Replace multiple agents with guidance: %s", contentStr),
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

// parseAgentGuidance 解析agent指导内容，支持字符串和JSON格式
// 字符串格式: "single guidance for one agent"
// JSON格式: {"Accommodation Planner": "Verify physical addresses...", "Restaurant Planner": "SKIP ALL MEALS..."}
// 返回: map[agentName]guidance
func (e *Environment) parseAgentGuidance(content interface{}) map[string]string {
	result := make(map[string]string)

	switch v := content.(type) {
	case string:
		// 如果是字符串，尝试解析为JSON
		err := json.Unmarshal([]byte(v), &result)
		if err != nil {
			// 如果JSON解析失败，返回空map（因为对于单个agent的情况，会在调用方处理）
			return make(map[string]string)
		}
	case map[string]interface{}:
		// 如果已经是map格式，直接转换
		for k, val := range v {
			if strVal, ok := val.(string); ok {
				result[k] = strVal
			}
		}
	default:
		// 其他类型，返回空map
		return make(map[string]string)
	}

	return result
}
