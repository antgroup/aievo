package aievo

import (
	"context"
	"fmt"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
)

func (e *AIEvo) BuildPlan(_ context.Context, _ string, _ ...llm.GenerateOption) (string, error) {
	// reserved, self define team member via LLM
	err := e.Team.InitSubRelation()
	return "", err
}

func (e *AIEvo) BuildSOP(ctx context.Context, prompt string, opts ...llm.GenerateOption) (string, error) {
	if e.SOP() == "" && e.SopExpert != nil {
		// execute sop agent, obtain sop
		gen, err := e.SopExpert.Run(ctx, []schema.Message{{
			Type:     schema.MsgTypeMsg,
			Content:  prompt,
			Sender:   _defaultSender,
			Receiver: e.SopExpert.Name(),
		}}, opts...)
		if err != nil {
			return "", err
		}

		// update cost
		_ = e.Produce(ctx, gen.Messages...)
	}

	return "", nil
}

func (e *AIEvo) Watch(ctx context.Context, _ string, opts ...llm.GenerateOption) (string, error) {
	// 开启一个 watcher 观察所有的执行流程，并给出评判建议，用于剔除和更新agent
	if e.Watcher != nil {
		e.WatchChan = make(chan schema.Message)
		e.WatchChanDone = make(chan struct{})
		go func() {
			for message := range e.WatchChan {
				if e.WatchCondition != nil && !e.WatchCondition(message) {
					e.WatchChanDone <- struct{}{}
					continue
				}
				generation, err := e.Watcher.Run(ctx,
					e.LoadMemory(ctx, e.Watcher), opts...)
				e.WatchChanDone <- struct{}{}
				if err != nil {
					continue
				}
				_ = e.Produce(ctx, generation.Messages...)
			}
		}()
	}
	return "", nil
}

func (e *AIEvo) Scheduler(ctx context.Context, prompt string, opts ...llm.GenerateOption) (string, error) {
	_ = e.Produce(ctx, schema.Message{
		Type:     schema.MsgTypeMsg,
		Content:  prompt,
		Sender:   _defaultSender,
		Receiver: e.GetTeamLeader().Name(),
	})
	for msg := e.Consume(ctx); msg != nil; msg = e.Consume(ctx) {
		if msg.IsEnd() {
			return msg.Content, nil
		}
		receivers := msg.Receivers()
		for _, rec := range receivers {
			receiver := e.Agent(rec)
			if receiver == nil {
				if len(receivers) == 1 {
					return msg.Content, fmt.Errorf(
						"get unexpected agent %s", msg.Receiver)
				}
				continue
			}
			messages := e.LoadMemory(ctx, receiver)
			if msg.Sender != _defaultSender {
				// Prepend the initial user prompt to the message history for every agent
				initialMessage := schema.Message{
					Type:     schema.MsgTypeMsg,
					Content:  prompt,
					Sender:   _defaultSender,
					Receiver: "All",
				}
				messages = append([]schema.Message{initialMessage}, messages...)
			}

			if e.Callback != nil {
				e.Callback.HandleAgentStart(ctx, receiver, messages)
			}
			gen, err := receiver.Run(ctx, messages, opts...) // gen 有 Messages and TotalTokens
			if err != nil {
				return "", err
			}
			if e.Callback != nil {
				e.Callback.HandleAgentEnd(ctx, receiver, gen)
			}

			if gen.Messages == nil {
				return "", fmt.Errorf("generating messages is nil for agent %s", msg.Receiver)
			}

			_ = e.Produce(ctx, gen.Messages...)
			e.broadcast(gen.Messages...) // 发给watcher
		}
	}
	return "", nil
}

func (e *AIEvo) broadcast(messages ...schema.Message) {
	if e.WatchChan == nil {
		return
	}
	for _, message := range messages {
		e.WatchChan <- message
		<-e.WatchChanDone
	}
}
