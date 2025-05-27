package driver

import (
	"context"

	"github.com/antgroup/aievo/schema"
	"github.com/goccy/go-graphviz/cgraph"
)

type Driver interface {
	IsInit() bool
	InitGraph(ctx context.Context, sop string) (Driver, error)
	UpdateGraphState(ctx context.Context, steps []schema.StepAction, actions []schema.StepAction) error
	GetCurrentNodes() []*cgraph.Node
	GetNextNodes(filterExecuted bool) []*cgraph.Node
	RenderStates() (current, next, all string)
	RenderCurrentGraph() (string, error)
	TmpRender() string
}

const (
	_executing      = "green"
	_done           = "red"
	_conditionShape = "diamond"
	_color          = "color"
	_shape          = "shape"
)
