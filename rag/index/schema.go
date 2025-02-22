package index

import (
	"context"
)

type WorkflowContext struct {
	basepath      string
	config        *WorkflowConfig
	Documents     []*Document
	TextUnits     []*TextUnit
	Relationships []*Relationship
	Entities      []*Entity
	Communities   []*Community
	Nodes         []*Node
	Reports       []*Report
}

type Progress func(ctx context.Context, args *WorkflowContext) error

type Document struct {
	Id          string
	Title       string
	Content     string
	TextUnitIds []string
}

type TextUnit struct {
	Id              string
	Text            string
	DocumentIds     []string
	EntityIds       []string
	RelationshipIds []string
	NumToken        int
}

type Entity struct {
	Id          string
	Title       string
	Type        string
	Desc        string
	TextUnitIds []string
}

type Relationship struct {
	Id             string
	Source         *Entity
	Target         *Entity
	Desc           string
	Weight         int
	CombinedDegree int
	TextUnitIds    []string
}

type Node struct {
	Id        string
	Title     string
	Community int
	Level     int
	Degree    int
}

type Community struct {
	Id              string
	Title           string
	Community       int
	Level           int
	RelationshipIds []string
	TextUnitIds     []string
	Parent          int
	EntityIds       []string
	Period          string
	Size            int
}

type Report struct {
}
