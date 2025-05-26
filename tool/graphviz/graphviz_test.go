package graphviz

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/antgroup/aievo/llm/openai"
	"github.com/goccy/go-graphviz"
	"github.com/thoas/go-funk"
)

func TestGraphviz(t *testing.T) {
	client, err := openai.New(
		openai.WithToken(os.Getenv("OPENAI_API_KEY")),
		openai.WithModel(os.Getenv("OPENAI_MODEL")),
		openai.WithBaseURL(os.Getenv("OPENAI_BASE_URL")))
	if err != nil {
		log.Fatal(err)
	}

	tool, err := NewGraphvizTool(client, 3)
	if err != nil {
		panic(err)
	}
	output, err := tool.Call(context.Background(), `
{
	"sop": "1. 产品根据用户的需求，写相关的需求文档，然后交给架构师
2. 架构师根据需求文档，写出系统设计的文档，然后交给项目经理
3. 项目经理根据系统设计文档，进行任务划分，然后分发任务给相应的程序员
4. 程序员A/B/C根据任务文档，写代码，交给测试员进行测试
5. 测试员对代码进行测试，如果通过，则结束流程，如果不通过，打回给程序员修改，修改后还需要交给测试员测试，重复该流程
6. 模块A/B/C都通过测试，项目完成",
	"agent_descriptions": "产品: 产品agent
架构师: 架构师agent
程序员: 程序员agent
测试: 测试agent"
}
`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("output: ")
	fmt.Println(output)
}

func TestGraphviz2(t *testing.T) {
	g, err := graphviz.ParseBytes([]byte(`
digraph ProcessFlow {

   node [shape=box, style=filled, color=lightblue];

   A [label="检查报警规则是否为连续2分钟及以上达到阈值"];
   B [label="检查报警的这台机器，在过去30分钟的报警指标是否有较大波动"];
   C [label="当前指标明显升高，报警有意义，理论上应该自愈"];
   D [label="检查报警触发时到触发后3分钟内是否有自愈记录"];
   E [label="查询自愈详情，确认是否执行自愈并提交工单"];
   G [label="检查应用是否有fgc相关的自愈规则"];
   H [label="明确报警有效，推荐用户重启，命令: restart server，推荐用户开启fgc自愈能力"];
   I [label="判断自愈的触发条件/阈值和报警触发条件(持续时间和阈值)是否匹配"];
   J [label="推理合理阈值和报警触发条件，建议修改报警和自愈规则，并明确报警有效，推荐用户重启，命令: restart server"];
   K [label="检查过去30分钟的ldc fgc与报警机器的差距"];
   L [label="推荐用户重启，命令: restart server"];
   M [label="给出结论，并告诉用户自愈拦截的原因，推荐用户重启，命令: restart server"];
   N [label="给出结论报警有效，自愈规则匹配，自愈会介入，人工无需介入"];


   A -> B [label="是"];
   A -> B [label="否，建议修改报警规则"];
   B -> C [label="有较大波动"];
   B -> K [label="无较大波动"];
   C -> D;
   D -> E [label="有自愈记录"];
   D -> G [label="无自愈记录"];
   E -> M [label="自愈被拦截"];
   G -> H [label="没有"];
   G -> I [label="有"];
   I -> J [label="不匹配"];
   I -> N [label="匹配"];
   K -> L [label="报警机器水位高3倍以上"];
}
`))
	if err != nil {
		log.Fatal(err)
	}

	nodeA, err := g.FirstNode()
	if err != nil {
		panic(err)
	}

	nodes := make([]*graphviz.Node, 0, 10)
	nodes = append(nodes, nodeA)
	for len(nodes) != 0 {
		length := len(nodes)
		for i := 0; i < length; i++ {
			edge, _ := g.FirstOut(nodes[i])
			name, _ := nodes[i].Name()
			label := nodes[i].Label()
			fmt.Printf("%s(%s)下游节点：\n", name, label)
			edges := make([]*graphviz.Edge, 0, 10)
			for edge != nil {
				nodes = append(nodes, edge.Node())
				edge.Node().Set("pos", "")
				edge.Node().Set("pos", "")
				n, _ := edge.Node().Name()
				fmt.Printf("- (%s): %s(%s)\n", edge.Label(), n,
					edge.Node().Label())
				edges = append(edges, edge)
				edge, _ = g.NextOut(edge)
			}
		}
		nodes = nodes[length:]
		nodes = funk.Uniq(nodes).([]*graphviz.Node)
	}

	g2, _ := graphviz.New(context.Background())
	var buf bytes.Buffer
	err = g2.Render(context.Background(), g, "dot", &buf)
	if err != nil {
		panic(err)
	}
	fmt.Println(buf.String())

}

func TestOutput(t *testing.T) {
	ctx := context.Background()
	g, err := graphviz.New(ctx)
	if err != nil {
		panic(err)
	}

	graph, err := g.Graph()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := graph.Close(); err != nil {
			panic(err)
		}
		g.Close()
	}()
	n, err := graph.CreateNodeByName("n")
	if err != nil {
		panic(err)
	}
	n.SetComment("this is n node")

	m, err := graph.CreateNodeByName("m")
	if err != nil {
		panic(err)
	}
	m.SetComment("this is m node")

	e, err := graph.CreateEdgeByName("e", n, m)
	if err != nil {
		panic(err)
	}
	e.SetLabel("e")

	var buf bytes.Buffer
	if err := g.Render(ctx, graph, "dot", &buf); err != nil {
		log.Fatal(err)
	}
	fmt.Println(buf.String())
}
