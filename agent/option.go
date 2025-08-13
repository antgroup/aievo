package agent

import (
	"github.com/antgroup/aievo/callback"
	"github.com/antgroup/aievo/driver"
	"github.com/antgroup/aievo/feedback"
	"github.com/antgroup/aievo/llm"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
)

type Option func(opt *Options)

const (
	_defaultMaxIterations = 20
)

type Options struct {
	prompt      string
	instruction string
	suffix      string

	name string
	desc string
	role string

	LLM              llm.LLM
	Tools            []tool.Tool
	useFunctionCall  bool
	FeedbackChain    feedback.Feedback
	Env              schema.Environment
	Callback         callback.Handler
	FilterMemoryFunc func([]schema.Message) []schema.Message
	ParseOutputFunc  func(string, *llm.Generation) ([]schema.StepAction, []schema.Message, error)
	Vars             map[string]string
	SOPGraph         string
	Driver           driver.Driver
	ReflectionPath   string // 反思文件路径

	MaxIterations int
}

func WithName(name string) Option {
	return func(opt *Options) {
		opt.name = name
	}
}

func WithDesc(desc string) Option {
	return func(opt *Options) {
		opt.desc = desc
	}
}

func WithRole(role string) Option {
	return func(opt *Options) {
		opt.role = role
	}
}

func WithPrompt(prompt string) Option {
	return func(opt *Options) {
		opt.prompt = prompt
	}
}

func WithInstruction(instruction string) Option {
	return func(opt *Options) {
		opt.instruction = instruction
	}
}

func WithSuffix(suffix string) Option {
	return func(opt *Options) {
		opt.suffix = suffix
	}
}

func WithLLM(LLM llm.LLM) Option {
	return func(opt *Options) {
		opt.LLM = LLM
	}
}

func WithTools(actions []tool.Tool) Option {
	return func(opt *Options) {
		opt.Tools = actions
	}
}

func WithUseFunctionCall(useFunctionCall bool) Option {
	return func(opt *Options) {
		opt.useFunctionCall = useFunctionCall
	}
}

func WithFeedbacks(feedbacks ...feedback.Feedback) Option {
	return func(opt *Options) {
		opt.FeedbackChain = feedback.Chain(feedbacks...)
	}
}

func WithEnv(env schema.Environment) Option {
	return func(opt *Options) {
		opt.Env = env
	}
}

func WithMaxIterations(maxIterations int) Option {
	return func(opt *Options) {
		opt.MaxIterations = maxIterations
	}
}

func WithCallback(callback callback.Handler) Option {
	return func(opt *Options) {
		opt.Callback = callback
	}
}

func WithVars(k, v string) Option {
	return func(opt *Options) {
		if opt.Vars == nil {
			opt.Vars = make(map[string]string)
		}
		opt.Vars[k] = v
	}
}

func WithSOPGraph(sop string) Option {
	return func(opt *Options) {
		opt.SOPGraph = sop
	}
}

func WithFilterMemoryFunc(fun func([]schema.Message) []schema.Message) Option {
	return func(opt *Options) {
		opt.FilterMemoryFunc = fun
	}
}

func WithParseOutputFunc(fun func(string, *llm.Generation) ([]schema.StepAction, []schema.Message, error)) Option {
	return func(opt *Options) {
		opt.ParseOutputFunc = fun
	}
}

func WithDriver(dri driver.Driver) Option {
	return func(opt *Options) {
		opt.Driver = dri
	}
}

func WithReflectionPath(path string) Option {
	return func(opt *Options) {
		opt.ReflectionPath = path
	}
}

func defaultBaseOptions() []Option {
	return []Option{
		WithPrompt(_defaultBasePrompt),
		WithInstruction(_defaultBaseInstructions),
		WithSuffix(_defaultBaseSuffix),
		WithMaxIterations(_defaultMaxIterations),
		WithFeedbacks(feedback.NewContentFeedback()),
		WithParseOutputFunc(parseOutput),
	}
}

func defaultGraphOptions() []Option {
	return []Option{
		WithPrompt(_defaultGraphPrompt),
		WithInstruction(_defaultGraphInstructions),
		WithSuffix(_defaultGraphSuffix),
		WithMaxIterations(_defaultMaxIterations),
		WithFeedbacks(feedback.NewContentFeedback()),
		WithParseOutputFunc(parseGraphOutput),
		WithDriver(driver.NewGraphDriver()),
	}
}

func defaultSopOptions() []Option {
	return []Option{
		WithName("SopExpert"),
		WithDesc("SopExpert"),
		WithPrompt(_defaultSopPrompt),
		WithInstruction(_defaultSopInstructions),
		WithSuffix(_defaultSopSuffix),
		WithMaxIterations(_defaultMaxIterations),
		WithParseOutputFunc(parseSopOutput),
	}
}

func defaultWatcherOptions() []Option {
	return []Option{
		WithName("WatcherAgent"),
		WithDesc("WatcherAgent"),
		WithPrompt(_defaultWatcherPrompt),
		WithInstruction(_defaultWatcherInstructions),
		WithSuffix(_defaultWatcherSuffix),
		WithMaxIterations(_defaultMaxIterations),
		WithParseOutputFunc(parseMngInfoOutput),
	}
}
