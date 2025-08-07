package environment

import (
	"context"

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
		e.Memory.RemoveMessagesByAgents(ctx, msg.MngInfo.Replace)
	}
	_ = e.Memory.Save(ctx, *msg)
	return nil
}

func (e *Environment) sopStrategy(ctx context.Context, msg *schema.Message) error {
	e.Sop = msg.Content
	e.Callback.HandleSOP(ctx, e.Sop)
	return nil
}
