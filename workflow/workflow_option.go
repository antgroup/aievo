package workflow

import "context"

// Option defines the option used to instantiate a Workflow.
// If an error occurs during instantiation, it needs to be reported. If no errors occurred, `nil` is returned.
type Option[TInput, TOutput any] func(d *Workflow[TInput, TOutput]) error

// NewWorkflow instantiates a workflow.
func NewWorkflow[TInput, TOutput any](options ...Option[TInput, TOutput]) (*Workflow[TInput, TOutput], error) {
	ctx, cancel := context.WithCancel(context.Background())
	dag := &Workflow[TInput, TOutput]{
		ctx:   ctx,
		close: cancel,
	}
	for _, option := range options {
		err := option(dag)
		if err != nil {
			return nil, err
		}
	}
	return dag, nil
}

// WithChannels specifies channel names for the workflow.
//
// Note that names cannot be repeated, and repeated names are counted once.
// All specified names must be used in the transit and may appear only once on the input and once on the output.
// At least two channel names must be specified, for input and output.
//
// This method is not required. Because WithTransits() will automatically add the channels mentioned by each transit
// to the channel list.
func WithChannels[TInput, TOutput any](names ...string) Option[TInput, TOutput] {
	return func(d *Workflow[TInput, TOutput]) error {
		if d.channels == nil {
			d.channels = NewWorkflowChannels()
		}
		return d.channels.add(names...)
	}
}

// WithChannelInput specifies the input channel for the entire workflow.
//
// When the workflow is executed, input data is injected from this channel.
// After the input data is injected into the channel, the workflow will be considered to start executing.
// This method can be executed multiple times. Those executed later will be merged with those executed earlier.
// The channel with the same name will be reinitialized.
func WithChannelInput[TInput, TOutput any](name string) Option[TInput, TOutput] {
	return func(d *Workflow[TInput, TOutput]) error {
		if d.channels == nil {
			d.channels = NewWorkflowChannels()
		}
		d.channels.channelInput = name
		return nil
	}
}

// WithChannelOutput specifies the final data output channel for the entire workflow.
//
// The final execution result should be injected into it. After the data is injected into the channel,
// the execution of the workflow is deemed to have ended. This method can be executed multiple times.
// Those executed later will be merged with those executed earlier.
// The channel with the same name will be reinitialized.
func WithChannelOutput[TInput, TOutput any](name string) Option[TInput, TOutput] {
	return func(d *Workflow[TInput, TOutput]) error {
		if d.channels == nil {
			d.channels = NewWorkflowChannels()
		}
		d.channels.channelOutput = name
		return nil
	}
}

// WithDefaultChannels defines the input and output channel for the workflow.
// Note that this method can be placed before or after the WithChannels(). But if it is placed before it,
// you need to ensure that the subsequent WithChannels() does not mention "input" and "output" again.
func WithDefaultChannels[TInput, TOutput any]() Option[TInput, TOutput] {
	return func(d *Workflow[TInput, TOutput]) error {
		if d.channels == nil {
			d.channels = NewWorkflowChannels()
		}
		err := d.channels.add("input", "output")
		if err != nil {
			return err
		}
		d.channels.channelInput = "input"
		d.channels.channelOutput = "output"
		return nil
	}
}

// WithTransits specifies specific nodes for the entire workflow.
//
// You can just call this method without calling WithChannels() to specify the input channel name.
// This method will automatically add unregistered channel names to the channel list.
//
// This method can be executed multiple times. Those executed later will be merged with those executed earlier.
//
// You need to ensure that the input channel name list involved in each transit does not intersect with
// the input channel name list of other transits, otherwise unpredictable consequences will occur during execution.
//
// So as the output channel list of every transit.
func WithTransits[TInput, TOutput any](transits ...TransitInterface) Option[TInput, TOutput] {
	return func(d *Workflow[TInput, TOutput]) error {
		lenTransits := len(transits)
		if lenTransits == 0 {
			return nil
		}
		if d.transits == nil {
			d.transits = &Transits{transits: make([]TransitInterface, 0)}
		}
		for _, t := range transits {
			for _, c := range t.GetChannelInputs() {
				if !d.channels.exists(c) {
					if err := d.channels.add(c); err != nil {
						return err
					}
				}
			}
			for _, c := range t.GetChannelOutputs() {
				if !d.channels.exists(c) {
					if err := d.channels.add(c); err != nil {
						return err
					}
				}
			}
			d.transits.transits = append(d.transits.transits, t)
		}
		return nil
	}
}

// WithLoggers specifies the Logger for the entire workflow.
//
// This method can be executed multiple times. The ones executed later will be merged with the ones executed earlier.
func WithLoggers[TInput, TOutput any](loggers ...LoggerInterface) Option[TInput, TOutput] {
	return func(d *Workflow[TInput, TOutput]) error {
		d.muLoggers.Lock()
		defer d.muLoggers.Unlock()
		if d.loggers == nil {
			d.loggers = &Loggers{}
		}
		for _, logger := range loggers {
			d.loggers.loggers = append(d.loggers.loggers, logger)
		}
		return nil
	}
}

// TransitOption defines the option used to instantiate a Transit.
type TransitOption func(d TransitInterface)

// LoggerOption defines the option used to instantiate a Logger.
type LoggerOption func(d *Logger)

// NewLogger instantiates a Logger. Its return value can be used as a parameter to WithLoggers().
func NewLogger(options ...LoggerOption) *Logger {
	logger := &Logger{}
	if len(options) == 0 {
		options = []LoggerOption{WithLoggerParams(LoggerParams{
			TimestampFormat: "2006-01-02 15:04:05.000000",
			Caller:          true})}
	}
	for _, option := range options {
		option(logger)
	}
	return logger
}

// WithLoggerParams defines the logger parameters.
func WithLoggerParams(params LoggerParams) LoggerOption {
	return func(d *Logger) {
		d.params = params
	}
}
