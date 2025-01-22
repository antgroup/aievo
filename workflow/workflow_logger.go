package workflow

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarning
	LevelError
)

// LogEventInterface represents the interface for log events. All log events need to implement the following three interfaces.
type LogEventInterface interface {
	// Name represents the name of the event.
	// The name is recommended to be a fixed enumeration value, such as the name of the transit.
	// The specific error type can be determined by its name. Empty or too long is not recommended.
	Name() string

	// Level represents the level of the event. The log level can refer to the already defined log level constant.
	Level() LogLevel

	// Message represents the message of the event. This content can be very detailed.
	Message() string
}

// LogEventErrorInterface represents the interface for log events for error.
type LogEventErrorInterface interface {
	// Error returns the error.
	Error() error
}

// LogEventTransitInterface represents the log event triggered by transit.
type LogEventTransitInterface interface {
	// Transit returns the transit that triggered the event. nil is not recommended.
	Transit() TransitInterface
}

// LogEventError provides a unified implementation for error events.
type LogEventError struct {
	LogEventErrorInterface
	// err records the error involved in the event.
	err error
}

// Error returns the error involved in the event.
func (l LogEventError) Error() error {
	return l.err
}

// LogEventTransit represents the event reported by transit.
type LogEventTransit struct {
	LogEventInterface
	LogEventTransitInterface
	// transit represents the one that triggered the event. nil is not recommended.
	transit TransitInterface
}

func (l LogEventTransit) Transit() TransitInterface {
	return l.transit
}

func (l LogEventTransit) Name() string {
	if l.transit == nil {
		return "<nil>"
	}
	return l.transit.Name()
}
func (l LogEventTransit) Level() LogLevel { return LevelDebug }

// LogEventTransitError represents an error reported by transit.
type LogEventTransitError struct {
	LogEventError
	LogEventTransit
}

func (l LogEventTransitError) Message() string {
	return l.LogEventError.err.Error()
}

func (l LogEventTransitError) Level() LogLevel {
	return LevelWarning
}

type LogEventWorkflow struct {
	LogEventInterface
}

func (l LogEventWorkflow) Name() string { return "<workflow>" }

type LogEventWorkflowError struct {
	LogEventError
	LogEventWorkflow
}

func (l LogEventWorkflowError) Message() string { return l.LogEventError.err.Error() }
func (l LogEventWorkflowError) Level() LogLevel { return LevelError }

// LogEventErrorValueTypeMismatch represents a value type mismatch error event.
type LogEventErrorValueTypeMismatch struct {
	LogEventTransitError
}

func (l LogEventErrorValueTypeMismatch) Message() string {
	return l.err.Error()
}

func (l LogEventErrorValueTypeMismatch) Level() LogLevel {
	return LevelError
}

// LogEventWorkflowStart indicates that the workflow starts execution.
type LogEventWorkflowStart struct {
	LogEventWorkflow
}

func (l LogEventWorkflowStart) Message() string { return "is starting..." }

func (l LogEventWorkflowStart) Level() LogLevel { return LevelDebug }

// LogEventWorkflowEnd indicates the end of workflow execution.
type LogEventWorkflowEnd struct {
	LogEventWorkflow
}

func (l LogEventWorkflowEnd) Message() string { return "ended." }

func (l LogEventWorkflowEnd) Level() LogLevel { return LevelDebug }

// LogEventTransitStart indicates that the transit starts execution.
type LogEventTransitStart struct {
	LogEventTransit
}

func (l LogEventTransitStart) Message() string {
	return "starting..."
}

// LogEventTransitEnd indicates the end of transit execution.
type LogEventTransitEnd struct {
	LogEventTransit
}

func (l LogEventTransitEnd) Message() string {
	return "ended."
}

// LogEventTransitCanceled indicates that transit received a cancellation signal.
type LogEventTransitCanceled struct {
	LogEventTransitError
}

func (l LogEventTransitCanceled) Message() string {
	return "cancellation notified."
}

// LogEventTransitWorkerPanicked indicates that panic() was triggered during the execution of the transit worker.
type LogEventTransitWorkerPanicked struct {
	LogEventTransitError
}

func (l LogEventTransitWorkerPanicked) Message() string {
	return "worker panicked."
}

func (l LogEventTransitWorkerPanicked) Level() LogLevel {
	return LevelError
}

// LogEventChannelReady represents the channel ready event.
type LogEventChannelReady struct {
	LogEventInterface
	value   any
	name    string
	message string
}

func (l LogEventChannelReady) Value() any {
	return l.value
}

func (l LogEventChannelReady) Name() string { return l.name }

func (l LogEventChannelReady) Level() LogLevel { return LevelDebug }

func (l LogEventChannelReady) Message() string {
	if l.message == "" {
		return "ready"
	}
	return l.message
}

// LogEventChannelInputReady represents the input channel ready event.
type LogEventChannelInputReady struct {
	LogEventChannelReady
}

func (l LogEventChannelInputReady) Message() string {
	if l.message == "" {
		return "->:channel"
	}
	return l.message
}

// LogEventChannelOutputReady represents the output channel ready event.
type LogEventChannelOutputReady struct {
	LogEventChannelReady
}

func (l LogEventChannelOutputReady) Message() string {
	if l.message == "" {
		return "<-:channel"
	}
	return l.message
}

// LoggerInterface defines the logging method and the parameters required by the logger.
// For specific usage, please refer to Logger.
type LoggerInterface interface {
	// Log an event.
	// ctx is the context in which the current method is called.
	// events can be one event or more.
	Log(ctx context.Context, events ...LogEventInterface)

	SetFlags(uint)
}

type LoggerParams struct {
	TimestampFormat string
	Caller          bool
	logDebugEnabled bool
}

type Logger struct {
	params LoggerParams
	LoggerInterface
}

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

const (
	// LDebugEnabled means displaying logs with the log level debug.
	LDebugEnabled = 0b10
)

func (l *Logger) SetFlags(flags uint) {
	l.params.logDebugEnabled = flags&LDebugEnabled > 0
}

func (l *Logger) logEvent(ctx context.Context, event LogEventInterface) {
	now := time.Now().Format(l.params.TimestampFormat)
	if !l.params.logDebugEnabled && (event.Level() == LevelDebug) {
		return
	}
	color := green
	std := os.Stdout
	if event.Level() == LevelWarning {
		color = yellow
		std = os.Stderr
	} else if event.Level() == LevelError {
		color = red
		std = os.Stderr
	}
	fmt.Fprintf(std, "[GO-DAG] %v |%s %10s %s| %s\n", now, color, event.Name(), reset, event.Message())
}

func (l *Logger) Log(ctx context.Context, events ...LogEventInterface) {
	for _, event := range events {
		l.logEvent(ctx, event)
	}
}

// ErrorCollectorInterface defines the methods that error collectors should implement.
// It is also a logger, so it also needs to implement all methods specified by the LoggerInterface.
type ErrorCollectorInterface interface {
	LoggerInterface
	// Listen starts listening goroutine.
	// ctx is the context of receiving "done signal".
	Listen(ctx context.Context)

	// Get returns all the log events of error reported by worker.
	Get() []LogEventErrorInterface

	// append receives the log event of error.
	append(event *LogEventErrorInterface)
}

// ErrorCollector is an error collector that collects errors in each worker report during the workflow execution,
// including panic().
// Due to a single execution of the workflow, the execution of the entire workflow is terminated when an error is
// reported. That is, if you only listen to the workflow execution once, you will only receive at most one error.
// But listening can continue, so you can keep listening for errors received when calling Execute() multiple times,
// you can also pass the same error collector to different workflows.
// For example, in a workflow that contains nested workflows, the same error collector can be passed in.
type ErrorCollector struct {
	ErrorCollectorInterface
	mu       sync.RWMutex
	errors   []LogEventErrorInterface
	listener chan LogEventErrorInterface
}

// NewErrorCollector instantiates a new error collector.
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors:   make([]LogEventErrorInterface, 0),
		listener: make(chan LogEventErrorInterface),
	}
}

// Listen starts a listening goroutine.
// ctx is the context of receiving "done signal".
// Since this method is an infinite loop, you need to call it asynchronously.
func (l *ErrorCollector) Listen(ctx context.Context) {
	var e LogEventErrorInterface
	for {
		select {
		case <-ctx.Done():
			return
		case e = <-l.listener:
			l.append(&e)
		default:
		}
	}
}

// Get returns all the log events of error reported by worker.
func (l *ErrorCollector) Get() []LogEventErrorInterface {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.errors
}

// append receives the log event of error.
func (l *ErrorCollector) append(event *LogEventErrorInterface) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, *event)
}

// Log an event.
func (l *ErrorCollector) Log(ctx context.Context, events ...LogEventInterface) {
	for _, event := range events {
		if e, ok := event.(LogEventErrorInterface); ok && event != nil {
			l.listener <- e
		}
	}
}

func (l *ErrorCollector) SetFlags(uint) {}
