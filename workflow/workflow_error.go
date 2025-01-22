package workflow

import (
	"fmt"
	"reflect"
	"strings"
)

// ErrChannelInterface indicates errors reported by the check channel.
type ErrChannelInterface interface {
	error
}

// ErrChannelNotInitialized indicates that the channel map is not instantiated.
type ErrChannelNotInitialized struct {
	ErrChannelInterface
}

func (e ErrChannelNotInitialized) Error() string {
	return "channel map not initialized"
}

// ErrChannelNotExist indicates that the specified channel does not exist.
type ErrChannelNotExist struct {
	ErrChannelInterface
	name string
}

func (e ErrChannelNotExist) Error() string {
	return fmt.Sprintf("channel[%s] not exist", e.name)
}

// ErrChannelInputNotSpecified indicates that the input channel is not specified.
type ErrChannelInputNotSpecified struct {
	ErrChannelInterface
}

func (e ErrChannelInputNotSpecified) Error() string {
	return "channel input not specified"
}

// ErrChannelOutputNotSpecified indicates that the output channel is not specified.
type ErrChannelOutputNotSpecified struct {
	ErrChannelInterface
}

func (e ErrChannelOutputNotSpecified) Error() string {
	return "channel output not specified"
}

// ErrChannelNameExisted indicates that the specified channel has existed.
type ErrChannelNameExisted struct {
	ErrChannelInterface
	name string
}

func (e ErrChannelNameExisted) Error() string {
	return fmt.Sprintf("the channel[%s] has existed.", e.name)
}

// ErrRedundantChannels indicates that there are unused channelInputs.
type ErrRedundantChannels struct {
	ErrChannelInterface
	channels []string
}

func (e ErrRedundantChannels) Error() string {
	return fmt.Sprintf("Redundant channelInputs: %v", strings.Join(e.channels, ", "))
}

// ErrTransitInterface represents the error reported by transit.
type ErrTransitInterface interface {
	error
}

// ErrWorkerPanicked reports when the worker is panicked.
type ErrWorkerPanicked struct {
	ErrTransitInterface
	panic any
}

func (e ErrWorkerPanicked) Error() string {
	return "worker panicked"
}

// ErrValueTypeMismatch defines that the data type output by the node is inconsistent with expectation.
type ErrValueTypeMismatch struct {
	ErrTransitInterface
	expect any
	actual any
	input  string
}

func (e ErrValueTypeMismatch) Error() string {
	return fmt.Sprintf("Expectation[%s] does not match actual[%s] received on channel[%s].",
		reflect.TypeOf(e.expect), reflect.TypeOf(e.actual), e.input)
}

// NewErrValueTypeMismatch instantiates an error to indicate that the type of received data on in input channel
// does not match the actual.
func NewErrValueTypeMismatch(expect, actual any, input string) ErrValueTypeMismatch {
	return ErrValueTypeMismatch{expect: expect, actual: actual, input: input}
}
