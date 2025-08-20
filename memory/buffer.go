package memory

import (
	"context"
	"fmt"
	"slices"

	"github.com/antgroup/aievo/schema"
)

type Buffer struct {
	Messages []schema.Message
	index    int
	window   int
}

func NewBufferMemory() *Buffer {
	return &Buffer{}
}

func NewBufferWindowMemory(window int) *Buffer {
	return &Buffer{window: window}
}

func (c *Buffer) Load(ctx context.Context, filter func(index, consumption int, message schema.Message) bool) []schema.Message {
	msgs := make([]schema.Message, 0, len(c.Messages))
	for i, message := range c.Messages {
		if filter == nil || filter(i, c.index, message) {
			msgs = append(msgs, message)
		}
	}
	if len(msgs) > c.window && c.window > 0 {
		msgs = msgs[len(msgs)-c.window:]
	}
	return msgs
}

func (c *Buffer) LoadNext(ctx context.Context, filter func(message schema.Message) bool) *schema.Message {
	if c.index >= len(c.Messages) {
		return nil
	}
	for ; c.index < len(c.Messages); c.index++ {
		if c.Messages[c.index].IsMsg() || c.Messages[c.index].IsEnd() ||
			c.Messages[c.index].IsCreative() {
			if c.Messages[c.index].Sender != c.Messages[c.index].Receiver {
				if filter != nil && !filter(c.Messages[c.index]) {
					return nil
				}
				c.index++
				return &c.Messages[c.index-1]
			}
		}
	}
	return nil
}

func (c *Buffer) Save(ctx context.Context, msg schema.Message) error {
	c.Messages = append(c.Messages, msg)
	return nil
}

func (c *Buffer) Clear(ctx context.Context) error {
	c.Messages = c.Messages[:0]
	return nil
}

func (c *Buffer) RemoveMessagesByAgents(ctx context.Context, agents []string) error {
	if len(agents) == 0 {
		return nil
	}

	// 用于跟踪每个agent是否已经保留了其收到的第一条"单独"消息
	firstSoloMessageKept := make(map[string]bool)
	for _, agentName := range agents {
		firstSoloMessageKept[agentName] = false
	}

	// 维护目标agent的接收者列表
	targetAgentReceivers := make(map[string]bool)

	newMessages := make([]schema.Message, 0, len(c.Messages))
	// 记录被删除agent收到的第一条消息在新列表中的位置
	firstMessageIndex := -1

	for _, msg := range c.Messages {
		shouldRemove := false

		// 规则 1: 检查消息是否由目标agent发送
		for _, agentName := range agents {
			if msg.Sender == agentName {
				shouldRemove = true
				// 将该消息的接收者添加到接收者列表中
				receivers := msg.Receivers()
				for _, receiver := range receivers {
					if receiver != agentName { // 避免自己给自己发消息的情况
						targetAgentReceivers[receiver] = true
					}
				}
				break
			}
		}

		// // 规则 1.5: 检查消息发送者是否在目标agent的接收者列表中
		// if !shouldRemove && targetAgentReceivers[msg.Sender] {
		// 	shouldRemove = true
		// }

		if shouldRemove {
			// 如果消息需要删除，则直接跳过，不添加到新列表
			continue
		}

		// 规则 2 & 3: 检查接收者逻辑
		receivers := msg.Receivers()
		// 如果有多个接收者，则保留消息
		if len(receivers) > 1 {
			// 不做任何事，shouldRemove 保持 false，消息将被保留
		} else if len(receivers) == 1 {
			// 如果只有一个接收者
			receiverName := receivers[0]
			isTargetAgent := slices.Contains(agents, receiverName)

			if isTargetAgent {
				// 如果这个唯一的接收者是目标agent
				if !firstSoloMessageKept[receiverName] {
					// 这是它收到的第一条单独消息，保留
					firstSoloMessageKept[receiverName] = true
					// 记录这条消息在新列表中的位置（作为重新开始的消费位点）
					if firstMessageIndex == -1 {
						firstMessageIndex = len(newMessages)
					}
				} else {
					// 这是后续的单独消息，删除
					shouldRemove = true
				}
			}
		}

		// 根据最终的标志决定是否保留消息
		if !shouldRemove {
			newMessages = append(newMessages, msg)
		}
	}

	fmt.Printf("Before removal: len(c.Messages) = %d, c.index = %d\n", len(c.Messages), c.index)
	c.Messages = newMessages

	// 将 c.index 指向被删除agent收到的第一条消息
	// 这样agent就会从"重新开始"的位置消费消息
	if firstMessageIndex != -1 {
		c.index = firstMessageIndex
	} else {
		// 如果没有找到第一条消息，保持原有索引（但要确保不越界）
		if c.index >= len(c.Messages) {
			c.index = len(c.Messages) - 1
		}
	}
	fmt.Printf("After removal: len(c.Messages) = %d, c.index = %d\n", len(c.Messages), c.index)
	return nil
}