package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
)

type UserProxyAgent struct {
	name    string
	desc    string
	command bool
}

func NewUserProxy(name, desc string, command bool) *UserProxyAgent {
	return &UserProxyAgent{
		name:    name,
		desc:    desc,
		command: command,
	}
}

func (a *UserProxyAgent) Run(ctx context.Context, messages []schema.Message, opts ...llm.GenerateOption) (*schema.Generation, error) {
	if messages == nil || len(messages) == 0 {
		return nil, errors.New("no messages provided")
	}

	message := messages[len(messages)-1]
	if !a.command {
		return &schema.Generation{
			Messages: []schema.Message{{
				Type:    schema.MsgTypeEnd,
				Content: message.Content,
			}},
			TotalTokens: 0,
		}, nil
	}
	fmt.Printf("%s: %s\nYou(input `exit/Enter` to exit):", message.Sender, message.Content)
	reader := bufio.NewReader(os.Stdin)
	content, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if strings.EqualFold(content, "exit") || strings.TrimSpace(content) == "" {
		return &schema.Generation{
			Messages: []schema.Message{{
				Type:    schema.MsgTypeEnd,
				Content: content,
			}},
			TotalTokens: 0,
		}, nil
	}
	receiver := message.Sender

	return &schema.Generation{
		Messages: []schema.Message{{
			Type:     schema.MsgTypeMsg,
			Content:  content,
			Receiver: receiver,
			Sender:   a.name,
		}},
		TotalTokens: 0,
	}, nil
}

func (a *UserProxyAgent) Name() string {
	return a.name
}

func (a *UserProxyAgent) Description() string {
	return a.desc
}

func (a *UserProxyAgent) WithEnv(_ schema.Environment) {
}

func (a *UserProxyAgent) Env() schema.Environment {
	return nil
}

func (a *UserProxyAgent) Tools() []tool.Tool {
	return nil
}
