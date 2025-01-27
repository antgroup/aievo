package feedback

import (
	"context"
	"fmt"
	"strings"

	"github.com/antgroup/aievo/schema"
)

type ContentFeedback struct{}

func NewContentFeedback() Feedback {
	return &ContentFeedback{}
}

func (*ContentFeedback) Feedback(_ context.Context, agent schema.Agent,
	messages []schema.Message, actions []schema.StepAction,
	steps []schema.StepAction, _ string) *FeedbackInfo {
	// 1. check response is correct
	if fd := checkResponseBlank(messages, actions); fd != nil {
		return fd
	}

	if fd := checkRepeatCall(steps, actions); fd != nil {
		return fd
	}

	if fd := checkMessages(agent, messages); fd != nil {
		return fd
	}

	return &FeedbackInfo{Type: Approved}
}

func checkResponseBlank(messages []schema.Message, actions []schema.StepAction) *FeedbackInfo {
	if len(messages) == 0 && len(actions) == 0 {
		return &FeedbackInfo{
			Type:  NotApproved,
			Msg:   fmt.Sprintf("no tool calls and no response return, please check your output content"),
			Token: 0,
		}
	}
	return nil
}

func checkRepeatCall(steps []schema.StepAction, actions []schema.StepAction) *FeedbackInfo {
	for _, action := range actions {
		for _, step := range steps {
			if step.Action == action.Action &&
				step.Input == action.Input &&
				step.Observation != "" {
				return &FeedbackInfo{
					Type: NotApproved,
					Msg:  fmt.Sprintf("this tool has been called before, do not repeate calls"),
				}

			}
		}
	}
	return nil
}

func checkMessages(agent schema.Agent, messages []schema.Message) *FeedbackInfo {
	for _, msg := range messages {
		if !msg.IsMsg() {
			continue
		}
		if msg.Receiver == "" {
			return &FeedbackInfo{
				Type: NotApproved,
				Msg:  "receiver cannot be empty where message is not END",
			}
		}
		if checkReceiver(agent, msg) {
			continue
		}
		return &FeedbackInfo{
			Type: NotApproved,
			Msg:  fmt.Sprintf("%s is not a valid agent name, please check your answer", msg.Receiver),
		}
	}
	return nil
}

func checkReceiver(agent schema.Agent, msg schema.Message) bool {
	if agent.Env() == nil {
		return true
	}
	agents := agent.Env().GetSubscribeAgents(
		context.Background(), agent)
	if len(agents) == 0 {
		return true
	}
	if strings.EqualFold(msg.Receiver, schema.MsgAllReceiver) {
		return true
	}
	for _, receiver := range msg.Receivers() {
		for _, a := range agents {
			if strings.EqualFold(a.Name(), strings.TrimSpace(receiver)) {
				return true
			}
		}
	}
	return false
}
