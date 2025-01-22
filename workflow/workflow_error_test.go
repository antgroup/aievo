package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrChannelNameExisted_Error(t *testing.T) {
	err := ErrChannelNameExisted{name: "test"}
	assert.Equal(t, "the channel[test] has existed.", err.Error())
}

func TestErrChannelInputNotSpecified_Error(t *testing.T) {
	err := ErrChannelInputNotSpecified{}
	assert.Equal(t, "channel input not specified", err.Error())
}

func TestErrChannelOutputNotSpecified_Error(t *testing.T) {
	err := ErrChannelOutputNotSpecified{}
	assert.Equal(t, "channel output not specified", err.Error())
}

func TestErrRedundantChannels_Error(t *testing.T) {
	err := ErrRedundantChannels{channels: []string{"test1", "test2"}}
	assert.Equal(t, "Redundant channelInputs: test1, test2", err.Error())
}

func TestNewErrValueTypeMismatch(t *testing.T) {
	err := ErrValueTypeMismatch{expect: 1, actual: 1.0, input: "test"}
	assert.Equal(t, "Expectation[int] does not match actual[float64] received on channel[test].", err.Error())
}

func TestErrChannelNotExist_Error(t *testing.T) {
	err := ErrChannelNotExist{name: "test"}
	assert.Equal(t, "channel[test] not exist", err.Error())
}

func TestErrWorkerPanicked_ErrorMessage(t *testing.T) {
	err := ErrWorkerPanicked{}
	assert.Equal(t, "worker panicked", err.Error())
}

func TestErrChannelNotInitialized_Error(t *testing.T) {
	var channels *Channels
	err := channels.add("input")
	assert.ErrorAs(t, err, &ErrChannelNotInitialized{})
	c, err := channels.get("input")
	assert.Nil(t, c)
	assert.ErrorAs(t, err, &ErrChannelNotInitialized{})
	channels = new(Channels)
	err = channels.add("input")
	assert.ErrorAsf(t, err, &ErrChannelNotInitialized{}, "channel map not initialized")
	assert.Equal(t, "channel map not initialized", err.Error())
}
