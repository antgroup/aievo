package workflow

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/antgroup/aievo/llm"
	prompt2 "github.com/antgroup/aievo/prompt"
	"github.com/antgroup/aievo/schema"
	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

type NodeType string

const (
	NodeTypeAgent     NodeType = "Agent"
	NodeTypeLLM       NodeType = "LLM"
	NodeTypeBranch    NodeType = "Branch"
	NodeTypeDirect    NodeType = "Direct"
	NodeTypeProcessor NodeType = "Processor"
	NodeTypeTool      NodeType = "Tool"
)

const (
	_jsonParse = "(?s)```json\n(.*?)\n```"
)

type ConditionalOutput struct {
	Output    string
	Condition string
}

type NodeOptions struct {
	name                   string
	agent                  schema.Agent
	llm                    llm.LLM
	processor              Processor
	tool                   tool.Tool
	nodeInputs             []string
	nodeOutputs            []string
	nodeConditionalOutputs []ConditionalOutput
}

type NodeOption func(*NodeOptions)

func WithName(name string) NodeOption {
	return func(o *NodeOptions) {
		o.name = name
	}
}

func WithAgent(agent schema.Agent) NodeOption {
	return func(o *NodeOptions) {
		o.agent = agent
	}
}

func WithLLM(llm llm.LLM) NodeOption {
	return func(o *NodeOptions) {
		o.llm = llm
	}
}

func WithProcessor(processor Processor) NodeOption {
	return func(o *NodeOptions) {
		o.processor = processor
	}
}

func WithTool(tool tool.Tool) NodeOption {
	return func(o *NodeOptions) {
		o.tool = tool
	}
}

func WithNodeInputs(inputs ...string) NodeOption {
	return func(o *NodeOptions) {
		o.nodeInputs = inputs
	}
}

func WithNodeOutputs(outputs ...string) NodeOption {
	return func(o *NodeOptions) {
		o.nodeOutputs = outputs
	}
}

func WithNodeConditionalOutputs(outputs ...ConditionalOutput) NodeOption {
	return func(o *NodeOptions) {
		o.nodeConditionalOutputs = outputs
	}
}

type dataTransferObject struct {
	fromNodeName   string
	fromBranchNode bool
	skipCurrent    bool
	selectedNodes  []string
	object         any
}

type DTO *dataTransferObject

func From(o any) DTO {
	return &dataTransferObject{
		fromNodeName:   "user",
		fromBranchNode: false,
		selectedNodes:  nil,
		object:         o,
	}
}

func FromNode(nodeName string, o any) DTO {
	return &dataTransferObject{
		fromNodeName:   nodeName,
		fromBranchNode: false,
		selectedNodes:  nil,
		object:         o,
	}
}

func Object(t DTO) any {
	return t.object
}

func convertDTO(a []any) []DTO {
	dtos := make([]DTO, len(a))
	for i, v := range a {
		dtos[i] = v.(DTO)
	}
	return dtos
}

func createSkipDTO() DTO {
	return &dataTransferObject{
		skipCurrent: true,
	}
}

func createBranchDTO(selectedNodes []string, input string) DTO {
	return &dataTransferObject{
		fromBranchNode: true,
		selectedNodes:  selectedNodes,
		object:         input,
	}
}

type BaseNode struct {
	Transit
	nodeType NodeType
}

func (n *BaseNode) NodeType() NodeType {
	return n.nodeType
}

func (n *BaseNode) handleInput(_ context.Context, a ...any) ([]DTO, bool) {
	args := convertDTO(a)
	dtos := make([]DTO, 0)
	for _, arg := range args {
		if (arg.fromBranchNode && !contains(arg.selectedNodes, n.name)) || arg.skipCurrent {
			continue
		}
		dtos = append(dtos, arg)
	}
	if len(dtos) == 0 {
		return nil, true
	}
	return dtos, false
}

type AgentNode struct {
	BaseNode
	agent schema.Agent
}

func (n *AgentNode) Agent() schema.Agent {
	return n.agent
}

func NewAgentNode(opts ...NodeOption) (*AgentNode, error) {
	options := &NodeOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if options.agent == nil {
		return nil, errors.New("agent is required")
	}

	node := &AgentNode{}
	initAgentNode(node, options)

	return node, nil
}

func initAgentNode(node *AgentNode, options *NodeOptions) {
	node.nodeType = NodeTypeAgent
	node.name = options.name
	node.inputTransits = make(map[string]TransitInterface)
	node.outputTransits = make(map[string]TransitInterface)
	node.agent = options.agent
	node.setChannelInputs(options.nodeInputs...)
	node.setChannelOutputs(options.nodeOutputs...)
	node.setWorker(node.agentWorker)
}

func (n *AgentNode) agentWorker(ctx context.Context, a ...any) (any, error) {
	dtos, noSelect := n.handleInput(ctx, a...)

	if noSelect {
		return createSkipDTO(), nil
	}

	prompt := extractInput(dtos)

	fmt.Println(fmt.Sprintf("[%s] input:\n%s", n.name, prompt))
	generation, _ := n.Agent().Run(ctx, []schema.Message{
		{
			Type:    schema.MsgTypeMsg,
			Content: prompt,
			Sender:  "User",
		},
	})
	content := generation.Messages[0].Content
	fmt.Println(fmt.Sprintf("[%s] output:\n%s", n.name, content))

	return FromNode(n.Name(), content), nil
}

type LLMNode struct {
	BaseNode
	llm llm.LLM
}

func (n *LLMNode) LLM() llm.LLM {
	return n.llm
}

func NewLLMNode(opts ...NodeOption) (*LLMNode, error) {
	options := &NodeOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if options.llm == nil {
		return nil, errors.New("llm is required")
	}

	node := &LLMNode{}
	initLLMNode(node, options)

	return node, nil
}

func initLLMNode(node *LLMNode, options *NodeOptions) {
	node.nodeType = NodeTypeLLM
	node.name = options.name
	node.llm = options.llm
	node.inputTransits = make(map[string]TransitInterface)
	node.outputTransits = make(map[string]TransitInterface)
	node.setChannelInputs(options.nodeInputs...)
	node.setChannelOutputs(options.nodeOutputs...)
	node.setWorker(node.llmWorker)
}

func (n *LLMNode) llmWorker(ctx context.Context, a ...any) (any, error) {
	dtos, noSelect := n.handleInput(ctx, a...)

	if noSelect {
		return createSkipDTO(), nil
	}

	prompt := extractInput(dtos)

	fmt.Println(fmt.Sprintf("[%s] input:\n%s", n.name, prompt))
	generate, err := n.LLM().Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	content := generate.Content
	fmt.Println(fmt.Sprintf("[%s] output:\n%s", n.name, content))
	return FromNode(n.Name(), content), nil
}

//go:embed prompts/branch_node.txt
var branchNodePrompt string

type Branch struct {
	condition string
	node      TransitInterface
}

type BranchNode struct {
	BaseNode
	llm      llm.LLM
	branches []Branch
	options  *NodeOptions
}

func NewBranchNode(opts ...NodeOption) (*BranchNode, error) {
	options := &NodeOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.llm == nil {
		return nil, errors.New("llm is required")
	}

	node := &BranchNode{}
	initBranchNode(node, options)

	return node, nil
}

func initBranchNode(node *BranchNode, options *NodeOptions) {
	node.nodeType = NodeTypeBranch
	node.name = options.name
	node.llm = options.llm
	node.inputTransits = make(map[string]TransitInterface)
	node.outputTransits = make(map[string]TransitInterface)
	node.setChannelInputs(options.nodeInputs...)
	nodeOutputs := make([]string, 0)
	for _, v := range options.nodeConditionalOutputs {
		nodeOutputs = append(nodeOutputs, v.Output)
	}
	node.setChannelOutputs(nodeOutputs...)
	node.setWorker(node.branchWorker)
	node.options = options
}

func (n *BranchNode) branchWorker(ctx context.Context, a ...any) (any, error) {
	dtos, noSelect := n.handleInput(ctx, a...)

	if noSelect {
		return createSkipDTO(), nil
	}

	for _, v := range n.options.nodeConditionalOutputs {
		branch := Branch{
			condition: v.Condition,
			node:      nil,
		}
		nextNode, ok := n.outputTransits[v.Output]
		if !ok {
			continue
		}
		branch.node = nextNode
		n.branches = append(n.branches, branch)
	}

	input := extractInput(dtos)

	args := make(map[string]any)
	args["branches"] = convertBranches(n.branches)
	args["input"] = input

	p, err := prompt2.NewPromptTemplate(branchNodePrompt)
	if err != nil {
		return nil, err
	}

	prompt, err := p.Format(args)
	if err != nil {
		return nil, err
	}

	generate, err := n.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	branch := convertLLMOutputToBranchName(generate.Content)
	if branch == "" {
		return nil, errors.New(fmt.Sprintf("[%s] no branch selected", n.name))
	}

	return createBranchDTO([]string{branch}, input), nil
}

func convertBranches(branches []Branch) string {
	var sb strings.Builder
	for _, v := range branches {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", v.node.Name(), v.condition))
	}
	return sb.String()
}

func convertLLMOutputToBranchName(output string) string {
	jsn := extractJSONContent(output)
	var result map[string]string
	err := json.Unmarshal([]byte(jsn), &result)
	if err != nil {
		return ""
	}
	return result["branch"]
}

func extractJSONContent(content string) string {
	compile := regexp.MustCompile(_jsonParse)
	submatch := compile.FindAllStringSubmatch(content, -1)
	if len(submatch) > 0 {
		return strings.TrimSpace(submatch[0][1])
	}
	return content
}

func extractInput(dtos []DTO) string {
	if len(dtos) == 0 {
		return ""
	}
	return dtos[0].object.(string)
}

func extractInputs(dtos []DTO) []string {
	var inputs []string
	for _, v := range dtos {
		inputs = append(inputs, v.object.(string))
	}
	return inputs
}

type DirectNode struct {
	BaseNode
}

func NewDirectNode(opts ...NodeOption) (*DirectNode, error) {
	options := &NodeOptions{}
	for _, opt := range opts {
		opt(options)
	}

	node := &DirectNode{}
	initDirectNode(node, options)

	return node, nil
}

func initDirectNode(node *DirectNode, options *NodeOptions) {
	node.nodeType = NodeTypeDirect
	node.name = options.name
	node.inputTransits = make(map[string]TransitInterface)
	node.outputTransits = make(map[string]TransitInterface)
	node.setChannelInputs(options.nodeInputs...)
	node.setChannelOutputs(options.nodeOutputs...)
	node.setWorker(node.directWorker)
}

func (n *DirectNode) directWorker(ctx context.Context, a ...any) (any, error) {
	dtos, noSelect := n.handleInput(ctx, a...)
	if noSelect {
		return createSkipDTO(), nil
	}
	input := extractInput(dtos)
	return FromNode(n.name, input), nil
}

type ProcessorNode struct {
	BaseNode
	processor Processor
}

func NewProcessorNode(opts ...NodeOption) (*ProcessorNode, error) {
	options := &NodeOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.processor == nil {
		return nil, errors.New("processor is required")
	}

	node := &ProcessorNode{}
	initProcessorNode(node, options)

	return node, nil
}

func initProcessorNode(node *ProcessorNode, options *NodeOptions) {
	node.nodeType = NodeTypeProcessor
	node.name = options.name
	node.processor = options.processor
	node.inputTransits = make(map[string]TransitInterface)
	node.outputTransits = make(map[string]TransitInterface)
	node.setChannelInputs(options.nodeInputs...)
	node.setChannelOutputs(options.nodeOutputs...)
	node.setWorker(node.processorWorker)
}

func (n *ProcessorNode) processorWorker(ctx context.Context, a ...any) (any, error) {
	dtos, noSelect := n.handleInput(ctx, a...)
	if noSelect {
		return createSkipDTO(), nil
	}
	inputs := extractInputs(dtos)
	output, err := n.processor.Process(ctx, inputs)
	if err != nil {
		return nil, err
	}
	return FromNode(n.Name(), output), nil
}

type ProcessorOptions struct {
	llm llm.LLM
}

type ProcessorOption func(*ProcessorOptions)

func WithLLMClient(llm llm.LLM) ProcessorOption {
	return func(o *ProcessorOptions) {
		o.llm = llm
	}
}

type Processor interface {
	Process(ctx context.Context, inputs []string) (string, error)
}

type JsonParser struct {
	Processor
}

func NewJsonParser(opts ...ProcessorOption) *JsonParser {
	options := &ProcessorOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return &JsonParser{}
}

func (p *JsonParser) Process(ctx context.Context, inputs []string) (string, error) {
	if len(inputs) == 0 {
		return "", errors.New("input is required")
	}

	if len(inputs) > 1 {
		return "", errors.New("too many inputs, this processor only support one input")
	}

	return extractJSONContent(inputs[0]), nil
}

//go:embed prompts/merge_data.txt
var mergeDataPrompt string

type LLMDataMerger struct {
	Processor
	llm llm.LLM
}

func NewLLMDataMerger(opts ...ProcessorOption) *LLMDataMerger {
	options := &ProcessorOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return &LLMDataMerger{
		llm: options.llm,
	}
}

func (m *LLMDataMerger) Process(ctx context.Context, inputs []string) (string, error) {
	if len(inputs) == 0 {
		return "", errors.New("input is required")
	}

	if len(inputs) == 1 {
		return inputs[0], nil
	}

	args := make(map[string]any)
	args["texts"] = convertTexts(inputs)

	p, err := prompt2.NewPromptTemplate(mergeDataPrompt)
	if err != nil {
		return "", err
	}

	prompt, err := p.Format(args)
	if err != nil {
		return "", err
	}

	generate, err := m.llm.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}

	return convertLLMOutputToMergeText(generate.Content), nil
}

func convertTexts(texts []string) string {
	var sb strings.Builder
	for i, v := range texts {
		sb.WriteString(fmt.Sprintf("text%d: %s\n", i+1, v))
	}
	return sb.String()
}

func convertLLMOutputToMergeText(output string) string {
	jsn := extractJSONContent(output)
	var result map[string]string
	err := json.Unmarshal([]byte(jsn), &result)
	if err != nil {
		return ""
	}
	return result["merged_text"]
}

type ToolNode struct {
	BaseNode
	tool tool.Tool
}

func NewToolNode(opts ...NodeOption) (*ToolNode, error) {
	options := &NodeOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.tool == nil {
		return nil, errors.New("tool is required")
	}

	node := &ToolNode{}
	initToolNode(node, options)

	return node, nil
}

func initToolNode(node *ToolNode, options *NodeOptions) {
	node.nodeType = NodeTypeTool
	node.name = options.name
	node.tool = options.tool
	node.inputTransits = make(map[string]TransitInterface)
	node.outputTransits = make(map[string]TransitInterface)
	node.setChannelInputs(options.nodeInputs...)
	node.setChannelOutputs(options.nodeOutputs...)
	node.setWorker(node.toolWorker)
}

func (n *ToolNode) toolWorker(ctx context.Context, a ...any) (any, error) {
	dtos, noSelect := n.handleInput(ctx, a...)
	if noSelect {
		return createSkipDTO(), nil
	}

	input := extractInput(dtos)

	output, err := n.tool.Call(ctx, input)
	if err != nil {
		return nil, err
	}

	return FromNode(n.Name(), output), nil
}

func contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}
