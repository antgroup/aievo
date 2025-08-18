package aievo

import (
	"context"

	"github.com/antgroup/aievo/environment"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/memory"
)

var (
	_defaultSender = "User"
)

func NewAIEvo(opts ...Option) (*AIEvo, error) {
	o := &options{
		maxTurn:  _defaultMaxTurn,
		maxToken: _defaultMaxToken,
		subMode:  environment.DefaultSubMode,
		env:      environment.NewEnv(),
	}

	for _, opt := range opts {
		opt(o)
	}

	e := &AIEvo{}

	initializeAIEvo(e, o)
	initializeEnvironment(e)
	initializeTeam(e, o)

	if e.GetTeamLeader() == nil {
		return nil, ErrMissingLeader
	}
	if len(e.GetTeam()) == 0 {
		return nil, ErrMissTeam
	}
	return e, nil
}

func initializeAIEvo(e *AIEvo, o *options) {
	e.Environment = o.env
	e.Callback = o.callback
	e.MaxToken = o.maxToken
	e.MaxTurn = o.maxTurn
	e.Team.Subscribes = o.subscribes
	e.Team.Leader = o.leader
	e.Sop = o.sop
	e.SopExpert = o.sopExpert
	e.Planner = o.planner
	e.Watcher = o.watcher
	e.WatchCondition = o.watchCondition
	// 设置默认的WatcherInterval为5，如果用户指定了则使用用户指定的值
	if o.watcherInterval > 0 {
		e.Environment.WatcherInterval = o.watcherInterval
	} else {
		e.Environment.WatcherInterval = 5 // 默认值
	}
	e.Handler = Chain(e.BuildPlan, e.BuildSOP, e.Watch, e.Scheduler)
}

func initializeEnvironment(e *AIEvo) {
	if e.Environment.Team == nil {
		e.Environment.Team = environment.NewTeam()
	}
	if e.Environment.Memory == nil {
		e.Environment.Memory = memory.NewBufferMemory()
	}
}

func initializeTeam(e *AIEvo, o *options) {
	e.Team.SubMode = o.subMode
	e.Team.AddMembers(o.team...)
	if o.user != nil {
		e.Team.AddMembers(o.user)
	}
	for _, member := range e.GetTeam() {
		if member.Env() == nil {
			member.WithEnv(e.Environment)
		}
	}
}

func (e *AIEvo) Run(ctx context.Context, prompt string, opts ...llm.GenerateOption) (string, error) {
	return e.Handler(ctx, prompt, opts...)
}
