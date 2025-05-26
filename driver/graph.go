package driver

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/antgroup/aievo/schema"
	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/thoas/go-funk"
)

type GraphDriver struct {
	init    bool
	Graph   *graphviz.Graph
	Current []*graphviz.Node
	Execute *graphviz.Graph
	nodes   []*graphviz.Node
}

func NewGraphDriver() *GraphDriver {
	return &GraphDriver{}
}

func (g *GraphDriver) IsInit() bool {
	return g.init
}

func (g *GraphDriver) InitGraph(_ context.Context, sop string) (Driver, error) {
	if sop == "" {
		return g, nil
	}
	graph, err := graphviz.ParseBytes([]byte(sop))
	if err != nil {
		return nil, err
	}
	g.Graph = graph
	g.Current = make([]*graphviz.Node, 0)
	execute, err := graphviz.New(context.Background())
	if err != nil {
		return nil, err
	}
	g.Execute, err = execute.Graph()
	if err != nil {
		return nil, err
	}
	g.init = true
	return g, nil
}

// UpdateGraphState action执行完成后置的操作
// 1. 判断action中的node，是否存在已执行的图中，不存在则添加
// 2. 判断 action 中的node的入边是否存在，不存在则添加
// 3. 判断 action 中的node的入度node的状态，并修改
// 4. 根据 steps，更新所有node的comment
func (g *GraphDriver) UpdateGraphState(ctx context.Context,
	steps []schema.StepAction, actions []schema.StepAction) error {

	nodes := make([]*graphviz.Node, 0, len(actions))
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		node, err := g.Graph.NodeByName(action.Node)
		if err != nil {
			return err
		}
		if node == nil {
			continue
		}
		nodes = append(nodes, node)
		names = append(names, action.Node)
	}
	g.Current = g.Current[0:0]
	err := g.updateNodes(ctx, nodes)
	if err != nil {
		return err
	}
	nodes = nodes[0:0]
	for _, name := range names {
		node, err := g.Execute.NodeByName(name)
		if err != nil {
			return err
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	g.Current = nodes
	err = g.updateEdges(ctx, nodes)
	if err != nil {
		return err
	}
	err = g.updateColor(ctx)
	if err != nil {
		return err
	}
	return g.updateComment(ctx, steps)
}

// updateNodes 从 Graph 的node里面，添加到 Execute 里面
func (g *GraphDriver) updateNodes(ctx context.Context, nodes []*cgraph.Node) error {
	// 1. 添加到新的图里
	for _, node := range nodes {
		name, _ := node.Name()
		newNode, err := g.Execute.NodeByName(name)
		if err != nil || newNode == nil {
			newNode, _ = g.Execute.CreateNodeByName(name)
			newNode.SetLabel(node.Label())
			newNode.SetShape(cgraph.Shape(node.GetStr(_shape)))
			newNode.SetColor(_executing)
			g.nodes = append(g.nodes, newNode)
		}
	}
	return nil
}

// updateEdges 从Graph里面，更新 Execute里面已有的Node的关系
// nodes 为 Execute 里面的 node
func (g *GraphDriver) updateEdges(ctx context.Context, nodes []*cgraph.Node) error {

	// 1. 添加到新的图里
	for _, node := range nodes {
		name, _ := node.Name()
		gNode, _ := g.Graph.NodeByName(name)
		// 2. 找到旧图里面所有跟这个节点相关的入节点
		edge, err := g.Graph.FirstIn(gNode)
		if err != nil {
			continue
		}
		for edge != nil {
			name, _ = edge.Node().Name()
			edgeName, _ := edge.Name()
			inNode, err := g.Execute.NodeByName(name)
			if err == nil && inNode != nil {
				// 说明存在入度的node，那么判断入度的node的状态
				// 如果入度的node和当前的node，不存在edge，则创建
				newEdge, _ := g.Execute.EdgeByName(edgeName, inNode,
					node)
				if newEdge == nil {
					newEdge, _ = g.Execute.CreateEdgeByName(
						edgeName, inNode, node)
					newEdge.SetLabel(edge.Label())
				}
			}
			edge, _ = g.Graph.NextIn(edge)
		}
	}
	return nil
}

// updateColor 遍历所有的节点，将完成的更新为done
// 根据节点的出度来进行判断
// 如果是条件分支节点，出度 >= 1 即可认为完成
// 如果非条件分支节点，Execute 中 当前节点的出度 == Graph 中出度，即可认为完成
func (g *GraphDriver) updateColor(_ context.Context) error {
	if len(g.nodes) == 0 {
		return nil
	}
	for _, node := range g.nodes {
		if node.GetStr(_color) == _done {
			continue
		}
		degree, _ := g.Execute.Outdegree(node)
		if node.GetStr(_shape) == _conditionShape {
			if degree >= 1 {
				node.SetColor(_done)
			} else {
				node.SetColor(_executing)
			}
			continue
		}
		name, _ := node.Name()
		gNode, _ := g.Graph.NodeByName(name)
		if gNode == nil {
			continue
		}
		gDegree, _ := g.Graph.Outdegree(gNode)
		if gDegree == degree {
			node.SetColor(_done)
		}
	}
	return nil

}

// updateComment 根据step 更新node的comment
func (g *GraphDriver) updateComment(ctx context.Context, steps []schema.StepAction) error {
	nodeStep := make(map[string]string)
	for i, step := range steps {
		nodeStep[step.Node] += fmt.Sprintf(`step %d:
Thought: %s
Action: %s, 
Action Input: %s
Observation: %s
`, i, step.Thought, step.Action, step.Input, step.Observation)
	}
	for name, step := range nodeStep {
		node, err := g.Execute.NodeByName(name)
		if err != nil {
			continue
		}
		if node == nil {
			fmt.Println("unknown node", name)
			continue
		}
		node.SetComment(step)
	}

	return nil

}

func (g *GraphDriver) UpdateBak(ctx context.Context) error {
	//
	// 	levels := make([]*graphviz.Node, 0, len(actions))
	// 	names := make([]string, 0, len(actions))
	// 	for _, action := range actions {
	// 		node, err := g.Graph.NodeByName(action.Node)
	// 		if err != nil {
	// 			return err
	// 		}
	// 		levels = append(levels, node)
	// 		names = append(names, action.Node)
	// 	}
	// 	g.Current = g.Current[0:0]
	// 	// 1. 添加到新的图里
	// 	for _, node := range levels {
	// 		name, _ := node.Name()
	// 		newNode, err := g.Execute.NodeByName(name)
	// 		if err != nil || newNode == nil {
	// 			newNode, _ = g.Execute.CreateNodeByName(name)
	// 			newNode.SetLabel(node.Label())
	// 			newNode.SetShape(cgraph.Shape(node.GetStr("shape")))
	// 		}
	// 		g.Current = append(g.Current, newNode)
	// 		// 2. 找到旧图里面所有跟这个节点相关的入节点
	// 		edge, err := g.Graph.FirstIn(node)
	// 		if err != nil {
	// 			continue
	// 		}
	// 		for edge != nil {
	// 			name, _ = edge.Node().Name()
	// 			edgeName, _ := edge.Name()
	// 			inNode, err := g.Execute.NodeByName(name)
	// 			if err == nil && inNode != nil {
	// 				// 说明存在入度的node，那么判断入度的node的状态
	// 				// 如果入度的node和当前的node，不存在edge，则创建
	// 				newEdge, _ := g.Execute.EdgeByName(edgeName, inNode,
	// 					newNode)
	// 				if newEdge == nil {
	// 					newEdge, _ = g.Execute.CreateEdgeByName(edgeName, inNode, newNode)
	// 					newEdge.SetLabel(edge.Label())
	// 				}
	// 			}
	// 			edge, _ = g.Graph.NextIn(edge)
	// 		}
	// 	}
	// 	// 3. 根据step和action 更新node的comment
	// 	nodeStep := make(map[string]string)
	// 	for i, step := range steps {
	// 		nodeStep[step.Node] += fmt.Sprintf(`step %d:
	// Thought: %s
	// Action: %s,
	// Action Input: %s
	// Observation: %s
	// `, i, step.Thought, step.Action, step.Input, step.Observation)
	// 	}
	// 	for name, step := range nodeStep {
	// 		node, err := g.Execute.NodeByName(name)
	// 		if err != nil {
	// 			continue
	// 		}
	// 		node.SetColor(_executing)
	// 		node.SetComment(step)
	// 	}
	// 	// 4. 遍历所有的节点，将完成的更新为done
	// 	node, _ := g.Graph.FirstNode()
	//
	// 	nodes := make([]*graphviz.Node, 0, 10)
	// 	nodes = append(nodes, node)
	// 	for len(nodes) != 0 {
	// 		length := len(nodes)
	// 		for i := 0; i < length; i++ {
	// 			color := _done
	// 			name, _ := nodes[i].Name()
	// 			executeNode, _ := g.Execute.NodeByName(name)
	// 			if executeNode == nil || executeNode.GetStr(_color) == _done {
	// 				continue
	// 			}
	// 			edge, _ := g.Graph.FirstOut(nodes[i])
	// 			for edge != nil {
	// 				nodes = append(nodes, edge.Node())
	// 				n, _ := edge.Node().Name()
	// 				newNode, _ := g.Execute.NodeByName(n)
	// 				if newNode != nil && (newNode.GetStr(_color) == _executing ||
	// 					newNode.GetStr(_color) == _done) {
	// 					edge, _ = g.Graph.NextOut(edge)
	// 					continue
	// 				}
	// 				color = _executing
	// 				break
	// 			}
	// 			executeNode.SetColor(color)
	// 		}
	// 		nodes = nodes[length:]
	// 		nodes = funk.Uniq(nodes).([]*graphviz.Node)
	// 	}

	return nil

}

func (g *GraphDriver) RenderStates() (current, next, all string) {
	nodes := g.GetCurrentNodes()
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		name, _ := node.Name()
		current += fmt.Sprintf("- %s: %s\n", name, node.Label())
		names = append(names, name)
	}

	for _, node := range g.GetNextNodes(true) {
		name, _ := node.Name()
		next += fmt.Sprintf("- %s: %s\n", name, node.Label())
		if !funk.Contains(names, name) {
			names = append(names, name)
		}
	}
	all = strings.Join(names, ",")
	return
}
func (g *GraphDriver) RenderStatesBak() (current, next, all string) {
	nodes := make([]string, 0)

	if len(g.Current) != 0 {
		for _, node := range g.Current {
			name, _ := node.Name()
			if funk.ContainsString(nodes, name) {
				continue
			}
			current += fmt.Sprintf("- %s: %s\n", name, node.Label())
			nodes = append(nodes, name)
		}
	}
	if len(g.Current) == 0 {
		node, _ := g.Graph.FirstNode()
		name, _ := node.Name()
		next += fmt.Sprintf("- %s: %s\n", name, node.Label())
		nodes = append(nodes, name)
	}
	if len(g.Current) != 0 {
		for _, node := range g.Current {
			name, _ := node.Name()
			node, _ = g.Graph.NodeByName(name)
			edge, _ := g.Graph.FirstOut(node)
			for edge != nil {
				name, _ = edge.Node().Name()
				if !funk.ContainsString(nodes, name) {
					next += fmt.Sprintf("- %s: %s\n",
						name, edge.Node().Label())
					nodes = append(nodes, name)
				}
				edge, _ = g.Graph.NextOut(edge)
			}
			edge, _ = g.Graph.FirstIn(node)
			for edge != nil {
				if edge.Node().GetStr("shape") == "diamond" {
					edge, _ = g.Graph.NextIn(edge)
					continue
				}
				outEdge, _ := g.Graph.FirstOut(edge.Node())
				for outEdge != nil {
					if outEdge.Node().GetStr(_color) ==
						_executing || outEdge.Node().GetStr(_color) ==
						_done {
						outEdge, _ = g.Graph.NextOut(outEdge)
						continue
					}
					name, _ = outEdge.Node().Name()
					if !funk.ContainsString(nodes, name) {
						next += fmt.Sprintf("- %s: %s\n",
							name, outEdge.Node().Label())
					}
					outEdge, _ = g.Graph.NextOut(outEdge)
				}
				edge, _ = g.Graph.NextIn(edge)
			}
		}
	}
	all = strings.Join(funk.UniqString(nodes), ", ")
	return
}

func (g *GraphDriver) GetCurrentNodes() []*cgraph.Node {
	nodes := make([]*cgraph.Node, 0)
	if len(g.Current) != 0 {
		for _, node := range g.Current {
			if funk.Contains(nodes, node) {
				continue
			}
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (g *GraphDriver) GetNextNodes(filterExecuted bool) []*cgraph.Node {
	nodes := make([]*cgraph.Node, 0)
	if len(g.Current) == 0 {
		node, _ := g.Graph.FirstNode()
		nodes = append(nodes, node)
		return nodes
	}
	for _, node := range g.Current {
		name, _ := node.Name()
		node, _ = g.Graph.NodeByName(name)
		edge, _ := g.Graph.FirstOut(node)
		for edge != nil {
			if !funk.Contains(nodes, edge.Node()) {
				nodes = append(nodes, edge.Node())
			}
			edge, _ = g.Graph.NextOut(edge)
		}
		edge, _ = g.Graph.FirstIn(node)
		for edge != nil {
			// 如果是条件分支，当前这个node，已经是其中一个分支的节点，另一个分支不用走
			if edge.Node().GetStr(_shape) == _conditionShape {
				edge, _ = g.Graph.NextIn(edge)
				continue
			}
			outEdge, _ := g.Graph.FirstOut(edge.Node())
			for outEdge != nil {
				if outEdge.Node().GetStr(_color) ==
					_executing || outEdge.Node().GetStr(_color) ==
					_done {
					outEdge, _ = g.Graph.NextOut(outEdge)
					continue
				}
				if !funk.Contains(nodes, outEdge.Node()) {
					nodes = append(nodes, outEdge.Node())
				}
				outEdge, _ = g.Graph.NextOut(outEdge)
			}
			edge, _ = g.Graph.NextIn(edge)
		}
	}

	if filterExecuted {
		filterNodes := make([]*cgraph.Node, 0, len(nodes))
		for _, node := range nodes {
			name, _ := node.Name()
			eNode, _ := g.Execute.NodeByName(name)
			if eNode == nil {
				filterNodes = append(filterNodes, node)
			}
		}
		nodes = filterNodes
	}
	return nodes
}

func (g *GraphDriver) RenderCurrentGraph() (string, error) {
	graph, err := graphviz.New(context.Background())
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = graph.Render(context.Background(),
		g.Execute, graphviz.XDOT, &buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (g *GraphDriver) TmpRender() string {
	m := make(map[*cgraph.Node]string)
	for _, node := range g.nodes {
		m[node] = node.GetStr("comment")
		node.SetComment("")
	}
	p, _ := g.RenderCurrentGraph()
	for _, node := range g.nodes {
		node.SetComment(m[node])
	}
	return p
}
