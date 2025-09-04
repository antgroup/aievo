package memory

import (
	"context"
	"fmt"

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

	// 1. 特殊处理 "ALL" 情况, 直接清空消息列表, 但保留用户消息
	if agents[0] == "ALL" {
		if len(c.Messages) > 0 {
			c.Messages = c.Messages[:1]
		}
		c.index = 0
		return nil
	}

	// 2. 检查目标agent是否已经发过消息
	agentHasSentMessage := false
	agentSet := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		agentSet[agent] = struct{}{}
	}

	for _, msg := range c.Messages {
		if _, ok := agentSet[msg.Sender]; ok {
			agentHasSentMessage = true
			break
		}
	}
	// 如果目标agent还没有发过消息，则直接返回
	if !agentHasSentMessage {
		return nil
	}

	// 3. 从头开始检测消息池里的消息，找到第一条接收者中有该agent的消息
	firstTargetMessageIndex := -1

	for i, msg := range c.Messages {
		receivers := msg.Receivers()
		// 检查接收者中是否有目标agent
		for _, agentName := range agents {
			for _, receiver := range receivers {
				if receiver == agentName {
					firstTargetMessageIndex = i
					break
				}
			}
			if firstTargetMessageIndex != -1 {     // 检查下一条消息是不是A-》c
				break
			}
		}
		if firstTargetMessageIndex != -1 {
			break
		}
	}

	fmt.Printf("Before removal: len(c.Messages) = %d, c.index = %d\n", len(c.Messages), c.index)

	// 如果找到了第一条目标消息，删除该消息之后的所有消息
	if firstTargetMessageIndex != -1 {
		// 保留从0到firstTargetMessageIndex的消息（包含该消息）
		c.Messages = c.Messages[:firstTargetMessageIndex+1]
		// 设置c.index指向该消息，使得下次调用时会重新处理这条消息
		c.index = firstTargetMessageIndex
		fmt.Printf("Found first target message at index %d, truncated messages after it\n", firstTargetMessageIndex)
	} else {
		// 如果没有找到目标消息，保持原状
		fmt.Printf("No target message found, keeping all messages\n")
	}

	fmt.Printf("After removal: len(c.Messages) = %d, c.index = %d\n", len(c.Messages), c.index)
	return nil
}
