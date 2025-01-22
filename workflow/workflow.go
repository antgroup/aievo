package workflow

import (
	"context"
	"sync"
)

// Interface defines a set of methods that a simple Workflow should implement.
//
// TInput represents the input data type, and TOutput represents the output data type.
type Interface[TInput, TOutput any] interface {
	// AddChannels adds additional channel names. All channels are unbuffered.
	// Each name can only appear once.
	// If it appears multiple times, subsequent occurrences of the same name will be ignored.
	AddChannels(names ...string) error

	// AddTransits adds additional transits of workflow.
	// Each name can only appear once.
	// If it appears multiple times, subsequent occurrences of the same name will be ignored.
	AddTransits(transits ...TransitInterface)

	// BuildWorkflow builds the workflow.
	//
	// Input and output channelInputs must be specified, otherwise error is returned.
	BuildWorkflow(ctx context.Context) error

	// BuildWorkflowInput builds channel(s) for inputting the execution result of the current node
	// to the subsequent node(s).
	//
	// Among the parameters, result is the content to be input to subsequent node(s).
	// inputs is the channelInputs of the channelInputs that receive the content.
	// You should check that the channel exists before calling this method, otherwise it will panic.
	BuildWorkflowInput(ctx context.Context, result any, inputs ...string)

	// BuildWorkflowOutput builds a set of channelInputs for receiving execution results of predecessor nodes.
	//
	// outputs is the channelInputs of the channelInputs that content will send to in order.
	// Only when all channelInputs specified by outputs have received data will the result be returned.
	// That is, as long as there is a channel that has not received data, the method will always be blocked.
	// The returned result is an array, the number of array elements is consistent with outputs,
	// and the results of its elements are in the order specified by outputs.
	// You should check that the channel exists before calling this method, otherwise it will panic.
	//
	// In particular, when ctx receives the end notification, it will return immediately,
	// but the content returned at this time is incomplete.
	// Therefore, you need to check the state of ctx immediately after receiving the return value of this method.
	// If the ctx status is `Done()`, the data returned by this method is not available.
	BuildWorkflowOutput(ctx context.Context, outputs ...string) *[]any

	// CloseWorkflow closes all channelInputs after workflow execution.
	//
	// Note: if you want to ensure that the workflow is only executed once,
	// execution of this method is deferred immediately after BuildWorkflow.
	CloseWorkflow()

	// Execute builds the workflow and executes it.
	// All channelInputs will not be closed after this method is executed, that is, you can execute it again and again.
	// But this method can only be executed once at a time.
	// If this method is called before the last call is completed, the call will be blocked until
	// the last execution ends. That is, pipelined execution is not supported.
	// If you want to execute multiple times simultaneously without blocking,
	// you should newly instantiate multiple Workflow instances and execute them separately.
	//   - ctx: The context of this execution.
	//          Note that if the current execution has not ended, when the method is called again,
	//          the parameter will replace the parameter of the previous execution.
	//          Therefore, it is strongly not recommended to call the method again
	//          when the previous execution has not ended, unless the parameter does not actually work.
	//   - input: The pointer to the input for the execution.
	Execute(ctx context.Context, input *TInput) *TOutput

	// RunOnce executes the workflow only once. All channelInputs are closed after execution.
	// If you want to re-execute, you need to re-create the workflow instance. (call NewWorkflow)
	//   - ctx: The context of this execution.
	//   - input: The pointer to the input for the execution.
	RunOnce(ctx context.Context, input *TInput) *TOutput

	// Cancel an executing workflow. If there are no workflows executing, there will be no impact.
	// If the context passed in during execution is cancelable,
	// it can also be canceled directly without calling this method.
	Cancel(cause error)

	// Log several logs.
	Log(ctx context.Context, events ...LogEventInterface)
}

// ChannelsInterface represents the interface that the workflow should implement.
type ChannelsInterface interface {
	exists(name string) bool
	get(name string) (chan any, error)
	add(names ...string) error
	close()
}

// Channels defines the channelInputs used by this directed acyclic graph.
type Channels struct {
	// channels stores all channels of this directed acyclic graph. The key of the map is the channel channels.
	channels   map[string]chan any
	muChannels sync.RWMutex
	// channelInput defines the channel used for input. Currently only a single input channel is supported.
	channelInput string
	// channelOutput defines the channel used for output. Currently only a single output channel is supported.
	channelOutput string

	ChannelsInterface
}

// NewWorkflowChannels instantiates a map of channels.
func NewWorkflowChannels() *Channels {
	return &Channels{
		channels: make(map[string]chan any),
	}
}

// exists checks if the name of channel exists.
func (d *Channels) exists(name string) bool {
	if d == nil || d.channels == nil {
		return false
	}
	if _, existed := d.channels[name]; existed {
		return true
	}
	return false
}

// get the specified name of channel.
// Before calling, you must ensure that the channel list has been initialized
// and the specified channel exists, otherwise an error will be reported.
func (d *Channels) get(name string) (chan any, error) {
	if d == nil {
		return nil, ErrChannelNotInitialized{}
	}
	// d.muChannels.RLock()
	//defer d.muChannels.RUnlock()
	if d.channels == nil {
		return nil, ErrChannelNotInitialized{}
	}
	if _, existed := d.channels[name]; !existed {
		return nil, ErrChannelNotExist{name: name}
	}
	return d.channels[name], nil
}

// add channels.
// Note that the channel name to be added cannot already exist. Otherwise, `ErrChannelNameExisted` will be returned.
func (d *Channels) add(names ...string) error {
	if d == nil || d.channels == nil {
		return ErrChannelNotInitialized{}
	}
	d.muChannels.Lock()
	defer d.muChannels.Unlock()
	for _, v := range names {
		v := v
		if d.exists(v) {
			return ErrChannelNameExisted{name: v}
		}
	}
	// Can only be added after all checks are passed.
	for _, v := range names {
		v := v
		d.channels[v] = make(chan any)
	}
	return nil
}

// close channels(only channelInput).
func (d *Channels) close() {
	close(d.channels[d.channelInput])
}

// ContextInterface represents the context interface that the workflow should implement.
type ContextInterface interface {
	// Cancel the workflow when executing. nil is not recommended. It is no impact when workflow is not executing.
	Cancel(cause error)
}

// Context defines the context, and the context must support cancellation reasons.
// When a node in the workflow reports an error during execution, the cancel function will be called to notify all nodes
// in the workflow to cancel the execution.
type Context struct {
	context context.Context
	cancel  context.CancelCauseFunc
	ContextInterface
}

// Cancel the execution.
// `nil` means no reason, but it is strongly recommended not to do this.
func (d *Context) Cancel(cause error) {
	d.cancel(cause)
}

// Loggers represents the loggers for the workflow.
type Loggers struct {
	muLoggers sync.RWMutex
	loggers   []LoggerInterface
}

func (d *Loggers) Log(ctx context.Context, events ...LogEventInterface) {
	d.muLoggers.RLock()
	defer d.muLoggers.RUnlock()
	if d.loggers == nil || len(d.loggers) == 0 {
		return
	}
	for _, logger := range d.loggers {
		if logger == nil {
			continue
		}
		go logger.Log(ctx, events...) // prevent the logger from blocking subsequent execution.
	}
}

// Workflow defines a generic directed acyclic graph of proposals.
// When you use it, you need to specify the input data type and output data type.
//
// Note that the input and output data types of the transit node are not mandatory,
// you need to verify it yourself.
type Workflow[TInput, TOutput any] struct {
	muChannels sync.RWMutex
	channels   *Channels
	muContext  sync.RWMutex
	context    *Context
	muTransits sync.RWMutex
	transits   *Transits
	muLoggers  sync.RWMutex
	loggers    *Loggers
	ctx        context.Context
	close      context.CancelFunc
	Interface[TInput, TOutput]
}

// Log several logs.
func (d *Workflow[TInput, TOutput]) Log(ctx context.Context, events ...LogEventInterface) {
	if events == nil || len(events) == 0 {
		return
	}
	d.muLoggers.RLock()
	defer d.muLoggers.RUnlock()
	if d.loggers == nil {
		return
	}
	d.loggers.Log(ctx, events...)
}

// AddChannels attaches the channels to the workflow.
// The parameter `channels` is the name list of the channels to be initialized for the first time, and the order of
// each element is irrelevant. If you don't pass any parameter, it will have no effect.
func (d *Workflow[TInput, TOutput]) AddChannels(channels ...string) error {
	if d.channels == nil {
		d.channels = NewWorkflowChannels()
	}
	if channels == nil || len(channels) == 0 {
		return nil
	}
	return d.channels.add(channels...)
}

// AddTransits attaches the transits to the workflow.
// The parameter transits represents a list of transit definitions, and the order of each element is irrelevant.
func (d *Workflow[TInput, TOutput]) AddTransits(transits ...TransitInterface) {
	if d.transits == nil {
		d.transits = &Transits{}
	}
	d.transits.transits = append(d.transits.transits, transits...)
}

// BuildWorkflowInput feeds the result to each input channel in turn.
//
// This function does not currently need to consider the termination of the superior context notification.
func (d *Workflow[TInput, TOutput]) BuildWorkflowInput(ctx context.Context, result any, inputs ...string) {
	d.muChannels.RLock()
	defer d.muChannels.RUnlock()
	var chs = make([]chan any, len(inputs))
	{
		d.muChannels.RLock()
		defer d.muChannels.RUnlock()

		if d.channels == nil {
			return
		}
		for i := 0; i < len(inputs); i++ {
			i := i
			if ch, err := d.channels.get(inputs[i]); err == nil {
				chs[i] = ch
			}
		}
	}
	for i := 0; i < len(inputs); i++ {
		i := i
		go func() {
			if chs[i] == nil {
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case chs[i] <- result:
					d.Log(ctx, LogEventChannelInputReady{LogEventChannelReady{value: result, name: inputs[i]}})
					return
				default:
				}
			}
			//if chs[i] != nil {
			//	d.Log(ctx, LogEventChannelInputReady{LogEventChannelReady{value: result, name: inputs[i]}})
			//	chs[i] <- result
			//}
		}()
	}
}

// BuildWorkflowOutput will wait for all channels specified by the current transit to output content,
// and then return the content together.
//
// If there is a channel that does not return in time, the current function will remain blocked.
// If you do not want to be blocked when calling the current method, you should use an asynchronous style.
// If a done notification is received from the superior context.Context, all coroutines will return immediately.
// At this point, `results[i]` that have not received data are still `nil`.
func (d *Workflow[TInput, TOutput]) BuildWorkflowOutput(ctx context.Context, outputs ...string) *[]any {
	var count = len(outputs)
	var results = make([]any, count)
	var chs = make([]chan any, count)
	{
		d.muChannels.RLock()
		defer d.muChannels.RUnlock()

		if d.channels == nil {
			return &results
		}
		for i := 0; i < count; i++ {
			i := i
			if ch, err := d.channels.get(outputs[i]); err == nil {
				chs[i] = ch
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		i := i
		go func() {
			defer wg.Done()
			if chs[i] == nil {
				return
			}
			for { // always check the done notification.
				select {
				case results[i] = <-chs[i]:
					d.Log(ctx, LogEventChannelOutputReady{LogEventChannelReady{value: results[i], name: outputs[i]}})
					return
				case <-ctx.Done():
					return // return immediately if done received and no longer wait for the channel.
				}
			}
		}()
	}
	wg.Wait()
	return &results
}

// BuildWorkflow is used to build the entire workflow.
//
// Note that before building, the input and output channels must be prepared, otherwise an exception will be returned:
//
// - ErrChannelNotInitialized if channels is nil
//
// - ErrChannelInputNotSpecified if channelInput is empty
//
// - ErrChannelOutputNotSpecified if channelOutput is empty
//
// If the transits is empty, do nothing and return nil directly.
// Otherwise, it is determined whether the input and output channel names mentioned in each transit are defined
// in the channel. As long as one channel does not exist, an error will be reported: ErrChannelNotExist.
//
// After the check passes, the workflow is built according to the following rules:
//
// - Call BuildWorkflowOutput to wait for the result of previous transit.
//
// - Running the worker of transit.
//
// - Call BuildWorkflowInput with the result of the worker.
func (d *Workflow[TInput, TOutput]) BuildWorkflow(ctx context.Context) error {
	d.muChannels.RLock()
	defer d.muChannels.RUnlock()
	if d.channels == nil || d.channels.channels == nil {
		return ErrChannelNotInitialized{}
	}

	d.channels.muChannels.RLock()
	defer d.channels.muChannels.RUnlock()
	// Checks the channel Input
	if len(d.channels.channelInput) == 0 {
		return ErrChannelInputNotSpecified{}
	}

	// Checks the channel Output
	if len(d.channels.channelOutput) == 0 {
		return ErrChannelOutputNotSpecified{}
	}

	d.transits.muTransits.RLock()
	defer d.transits.muTransits.RUnlock()
	// Checks the Transits
	if len(d.transits.transits) == 0 {
		return nil
	}
	for _, t := range d.transits.transits {
		for _, name := range t.GetChannelInputs() {
			if _, existed := d.channels.channels[name]; !existed {
				return ErrChannelNotExist{name: name}
			}
		}
		for _, name := range t.GetChannelOutputs() {
			if _, existed := d.channels.channels[name]; !existed {
				return ErrChannelNotExist{name: name}
			}
		}
	}

	for _, t := range d.transits.transits {
		go func(ctx context.Context, t TransitInterface) {
			// Waiting for the results of input channels to be ready.
			// If there is a channel with no output, the current coroutine will be blocked here.
			// If done notification is received, return immediately and no longer wait for the channel.
			results := d.BuildWorkflowOutput(ctx, t.GetChannelInputs()...)
			select {
			case <-ctx.Done(): // If the cancellation notification has been received, it will exit directly.
				d.Log(ctx, LogEventTransitCanceled{
					LogEventTransitError: LogEventTransitError{
						LogEventTransit: LogEventTransit{transit: t},
						LogEventError:   LogEventError{err: ctx.Err()},
					}})
				return
			default:
				//log.Println("build workflow:", t.channelInputs, " selected.")
			}
			// The sub-Context is derived here only to prevent other Contexts from being affected when the worker stops
			// actively.
			workerCtx, _ := context.WithCancelCause(ctx)
			var work = func(t TransitInterface) (any, error) {
				d.Log(ctx, LogEventTransitStart{LogEventTransit: LogEventTransit{transit: t}})
				defer d.Log(ctx, LogEventTransitEnd{LogEventTransit: LogEventTransit{transit: t}})
				return t.Run(workerCtx, *results...)
			}
			var result, _ = func(t TransitInterface) (any, error) {
				defer func() {
					if err := recover(); err != nil {
						e := ErrWorkerPanicked{panic: err}
						d.Log(ctx, LogEventTransitWorkerPanicked{
							LogEventTransitError: LogEventTransitError{
								LogEventTransit: LogEventTransit{transit: t},
								LogEventError:   LogEventError{err: e}}})
						if !t.GetAllowFailure() {
							d.context.Cancel(&e)
							return
						}
					}
				}()
				result, err := work(t)
				if err != nil {
					d.Log(ctx, LogEventTransitError{
						LogEventTransit: LogEventTransit{transit: t},
						LogEventError:   LogEventError{err: err}})
				}
				if err != nil && !t.GetAllowFailure() {
					d.context.Cancel(err)
					return nil, err
				}
				return result, err
			}(t)
			d.BuildWorkflowInput(ctx, result, t.GetChannelOutputs()...)
		}(ctx, t)
	}
	return nil
}

// CloseWorkflow closes the workflow after execution.
// After this method is executed, all input, output channels and transits will be deleted.
// Note, please do not call this method during workflow execution, otherwise it will lead to unpredictable consequences.
func (d *Workflow[TInput, TOutput]) CloseWorkflow() {
	d.close()
}

// Execute the workflow.
//
// root is the highest-level context.Context for this execution. Each transit worker will receive its child context.
//
// Note that if two tasks with different durations are executed in a short period of time, there is no guarantee that
// the execution results will be as expected, because the input of the subsequent task may be sent to the channel
// before the previous task. If you want to execute multiple identical workflows in a short period of time
// without unpredictable data transfer order, please instantiate a new workflow before each execution.
func (d *Workflow[TInput, TOutput]) Execute(root context.Context, input *TInput) *TOutput {
	// The sub-context is introduced and has a cancellation handler, making it easy to terminate the entire process
	// at any time.
	select {
	case <-d.ctx.Done():
		return nil
	default:
	}
	ctx, cancel := context.WithCancelCause(root)

	// Record the context and cancellation handler, so they can be called at the appropriate time.
	d.muContext.Lock()
	d.context = &Context{context: ctx, cancel: cancel}
	d.muContext.Unlock()

	d.Log(ctx, LogEventWorkflowStart{})
	err := d.BuildWorkflow(ctx)
	if err != nil {
		d.Log(ctx, LogEventWorkflowError{LogEventError: LogEventError{err: err}})
		return nil
	}
	var results *TOutput
	signal := make(chan struct{})
	go func(ctx context.Context) {
		defer func() {
			signal <- struct{}{}
		}()
		r := d.BuildWorkflowOutput(ctx, d.channels.channelOutput)
		select {
		case <-ctx.Done(): // If the end notification has been received, it will exit directly.
			return
		default:
		}
		if ra, ok := (*r)[0].(TOutput); ok {
			results = &ra
		} else {
			var a = new(TOutput)
			var e = ErrValueTypeMismatch{actual: (*r)[0], expect: *a, input: d.channels.channelOutput}
			d.Log(ctx, LogEventErrorValueTypeMismatch{
				LogEventTransitError: LogEventTransitError{
					LogEventError: LogEventError{err: e}}})
			results = nil
		}
	}(ctx)
	d.BuildWorkflowInput(ctx, *input, d.channels.channelInput)
	<-signal
	d.Log(ctx, LogEventWorkflowEnd{})
	return results
}

// RunOnce executes the workflow only once and destroys it. Calling Execute() or RunOnce() again will report an error.
func (d *Workflow[TInput, TOutput]) RunOnce(ctx context.Context, input *TInput) *TOutput {
	defer d.CloseWorkflow()
	return d.Execute(ctx, input)
}

// Cancel an executing workflow. If there are no workflow executing, there will be no impact.
func (d *Workflow[TInput, TOutput]) Cancel(cause error) {
	d.muContext.RLock()
	defer d.muContext.RUnlock()
	if d.context == nil {
		return
	}
	d.context.Cancel(cause)
}
