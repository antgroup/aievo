package aievo

import (
	"errors"

	"github.com/antgroup/aievo/callback"
	"github.com/antgroup/aievo/environment"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
)

var (
	ErrMissingLeader = errors.New("leader agent is not set")
	ErrMissTeam      = errors.New("team is not set")
)

const (
	_defaultMaxTurn  = 20
	_defaultMaxToken = 200 * 4096
)

type options struct {
	team           []schema.Agent
	leader         schema.Agent
	env            *environment.Environment
	subscribes     []schema.Subscribe
	maxTurn        int
	maxToken       int
	subMode        environment.SubscribeMode
	LLM            llm.LLM
	user           schema.Agent
	callback       callback.Handler
	sopExpert      schema.Agent
	planner        schema.Agent
	watcher        schema.Agent
    watchCondition func(message schema.Message, memory schema.Memory) bool

	sop string
}

type Option func(options *options)

func WithTeam(agents []schema.Agent) Option {
	return func(opts *options) {
		opts.team = agents
	}
}

func WithTeamLeader(leader schema.Agent) Option {
	return func(opts *options) {
		opts.leader = leader
	}
}

func WithSubScribeMode(mode environment.SubscribeMode) Option {
	return func(opts *options) {
		opts.subMode = mode
	}
}

func WithEnvironment(env *environment.Environment) Option {
	return func(opts *options) {
		opts.env = env
	}
}

func WithCallback(handler callback.Handler) Option {
	return func(opts *options) {
		opts.callback = handler
	}
}

func WithSubscribe(subscribed schema.Agent,
	subscribers ...schema.Agent) Option {
	return func(opts *options) {
		for _, sub := range subscribers {
			if sub == subscribed {
				continue
			}
			opts.subscribes = append(opts.subscribes,
				schema.Subscribe{
					Subscribed: subscribed,
					Subscriber: sub,
				})
		}
	}
}

func WithConditionSubscribe(subscribed schema.Agent, condition string,
	subscribers ...schema.Agent) Option {
	return func(opts *options) {
		for _, subscriber := range subscribers {
			if subscriber == subscribed {
				continue
			}
			opts.subscribes = append(opts.subscribes,
				schema.Subscribe{
					Subscribed: subscribed,
					Subscriber: subscriber,
					Condition:  condition,
				})
		}
	}
}

func WithMaxTurn(maxTurn int) Option {
	return func(opts *options) {
		opts.maxTurn = maxTurn
	}
}

func WithMaxToken(maxToken int) Option {
	return func(opts *options) {
		opts.maxToken = maxToken
	}
}

func WithLLM(llm llm.LLM) Option {
	return func(opts *options) {
		opts.LLM = llm
	}
}

func WithUserProxy(user schema.Agent) Option {
	return func(opts *options) {
		opts.user = user
	}
}

func WithSOP(sop string) Option {
	return func(opts *options) {
		opts.sop = sop
	}
}

func WithSopExpert(agent schema.Agent) Option {
	return func(opts *options) {
		opts.sopExpert = agent
	}
}

func WithWatcher(agent schema.Agent, condition func(message schema.Message, memory schema.Memory) bool) Option {
	return func(opts *options) {
		opts.watcher = agent
		opts.watchCondition = condition
	}
}

func WithSubMode(mode environment.SubscribeMode) Option {
	return func(opts *options) {
		opts.subMode = mode
	}
}