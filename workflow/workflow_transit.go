package workflow

import (
	"context"
	"sync"
)

// TransitInterface represents the interface that transit should implement.
type TransitInterface interface {
	// Name of the transit node.
	//
	// Although it is not required to be unique, it is not recommended to duplicate the name or leave it blank,
	// and it is not recommended to use `input` or `output` because it may be a reserved name in the future.
	Name() string

	// GetChannelInputs defines the input channel names required by this transit node.
	//
	// All ChannelInputs are listened to when building a workflow. Each channel is unbuffered.
	// The transit node will only be executed when all ChannelInputs have passed in values.
	// Therefore, you need to ensure that each channel must have data coming in, otherwise it will always be blocked.
	GetChannelInputs() []string

	// setChannelInputs sets several channel names for receiving data.
	setChannelInputs(...string)

	// GetChannelOutputs defines the output channel names required by this transit node.
	//
	// The results of the execution of the transit node will be sequentially sent to the designated output channel,
	// so that the subsequent node can continue to execute. Therefore, you need to ensure that each channel must
	// have and only one receiver, otherwise it will always be blocked.
	GetChannelOutputs() []string

	// setChannelOutputs sets several channel names the working result to be fed for.
	setChannelOutputs(...string)

	// GetAllowFailure indicates whether to continue when the worker reports an error or panicked. If true returned,
	// it means that the worker error or panic will not affect subsequent execution,
	// otherwise the execution should be interrupted. Default is false.
	GetAllowFailure() bool

	// setAllowFailure sets the "allow failure" attribute.
	// See GetAllowFailure.
	setAllowFailure(allow bool)

	// Run represents the running worker of this transit.
	//
	// The first parameter is context.Context, which is used to receive the context derived from the superior to
	// the current task. All contexts within a worker can receive done notifications for this context.
	// If your worker is time-consuming, it should be able to receive incoming done notifications from the context.
	//
	// The next parameter list is the parameters passed in by the channel defined by ChannelInputs. You need to check
	// the type of each parameter passed in yourself.
	//
	// The first return value is the execution result of the current worker. If it is a complex value, it is recommended
	// to pass a pointer. It will be sent to each channel defined by ChannelOutputs in order.
	// You need to ensure the correctness of the parameter and return value types by yourself, otherwise it will panic.
	//
	// worker can return error, but error are not passed on to subsequent transits.
	// If the worker returns an error other than nil, the workflow will be canceled directly.
	//
	// It is strongly discouraged to panic() in the worker, as this will cause the process to abort and
	// cannot be recovered.
	Run(context.Context, ...any) (any, error)

	// setWorker sets the worker for the current transit for the Run to call.
	setWorker(func(ctx context.Context, a ...any) (any, error))

	// addInputTransit add input relationship between transit and chan.
	addInputTransit(transit TransitInterface, chanName string)

	// addOutputTransit add output relationship between transit and chan.
	addOutputTransit(chanName string, transit TransitInterface)
}

// Transit defines the transit node of a directed acyclic graph.
type Transit struct {
	TransitInterface
	name           string
	channelInputs  []string
	channelOutputs []string
	allowFailure   bool
	worker         func(context.Context, ...any) (any, error)
	inputTransits  map[string]TransitInterface
	outputTransits map[string]TransitInterface
}

func (t *Transit) Name() string {
	return t.name
}

func (t *Transit) GetChannelInputs() []string {
	return t.channelInputs
}

func (t *Transit) setChannelInputs(names ...string) {
	t.channelInputs = append(t.channelInputs, names...)
}

func (t *Transit) GetChannelOutputs() []string {
	return t.channelOutputs
}

func (t *Transit) setChannelOutputs(names ...string) {
	t.channelOutputs = append(t.channelOutputs, names...)
}

func (t *Transit) GetAllowFailure() bool {
	return t.allowFailure
}

func (t *Transit) setAllowFailure(allow bool) {
	t.allowFailure = allow
}

func (t *Transit) Run(ctx context.Context, a ...any) (any, error) {
	return t.worker(ctx, a...)
}

func (t *Transit) setWorker(worker func(ctx context.Context, a ...any) (any, error)) {
	t.worker = worker
}

func (t *Transit) addInputTransit(transit TransitInterface, chanName string) {
	t.inputTransits[chanName] = transit
}

func (t *Transit) addOutputTransit(chanName string, transit TransitInterface) {
	t.outputTransits[chanName] = transit
}

// Transits represents the transits for the workflow.
type Transits struct {
	muTransits sync.RWMutex
	transits   []TransitInterface
}

// NewTransit instantiates the workflow's transit.
func NewTransit(name string, options ...TransitOption) TransitInterface {
	transit := &Transit{
		name:           name,
		inputTransits:  make(map[string]TransitInterface),
		outputTransits: make(map[string]TransitInterface),
	}
	for _, option := range options {
		option(transit)
	}
	return transit
}

// WithInputs specifies the input channels to be monitored for transit.
// Note that the channel names passed in is consistent with the number and order of parameters
// eventually received by the worker.
// This method can be called multiple times,
// but the results of subsequent calls will be merged and overwritten by previous calls.
func WithInputs(names ...string) TransitOption {
	return func(d TransitInterface) {
		d.setChannelInputs(append(d.GetChannelInputs(), names...)...)
	}
}

// WithOutputs specifies the channel names for transit to be injected into worker execution results.
// Each channel injects the same content, so the order of channel names passed in does not matter.
// This method can be called multiple times,
// but the results of subsequent calls will be merged and overwritten by previous calls.
func WithOutputs(names ...string) TransitOption {
	return func(d TransitInterface) {
		d.setChannelOutputs(append(d.GetChannelOutputs(), names...)...)
	}
}

// WithWorker specifies a worker for transit.
// This method can be called multiple times, but only the last one takes effect.
func WithWorker(worker func(context.Context, ...any) (any, error)) TransitOption {
	return func(d TransitInterface) {
		d.setWorker(worker)
	}
}

// WithAllowFailure specifies whether to continue when the worker reports an error or panicked for the transit.
// This method can be called multiple times, but only the last one takes effect.
func WithAllowFailure(allow bool) TransitOption {
	return func(d TransitInterface) {
		d.setAllowFailure(allow)
	}
}
