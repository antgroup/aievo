import argparse
import json
import networkx as nx
import pandas as pd
from graspologic.partition import hierarchical_leiden
import html
from typing import Any, cast
from graspologic.utils import largest_connected_component


def normalize_node_names(graph: nx.Graph | nx.DiGraph) -> nx.Graph | nx.DiGraph:
    """Normalize node names."""
    node_mapping = {node: html.unescape(node.upper().strip()) for node in graph.nodes()}  # type: ignore
    return nx.relabel_nodes(graph, node_mapping)


def _stabilize_graph(graph: nx.Graph) -> nx.Graph:
    """Ensure an undirected graph with the same relationships will always be read the same way."""
    fixed_graph = nx.DiGraph() if graph.is_directed() else nx.Graph()

    sorted_nodes = graph.nodes(data=True)
    sorted_nodes = sorted(sorted_nodes, key=lambda x: x[0])

    fixed_graph.add_nodes_from(sorted_nodes)
    edges = list(graph.edges(data=True))

    # If the graph is undirected, we create the edges in a stable way, so we get the same results
    # for example:
    # A -> B
    # in graph theory is the same as
    # B -> A
    # in an undirected graph
    # however, this can lead to downstream issues because sometimes
    # consumers read graph.nodes() which ends up being [A, B] and sometimes it's [B, A]
    # but they base some of their logic on the order of the nodes, so the order ends up being important
    # so we sort the nodes in the edge in a stable way, so that we always get the same order
    if not graph.is_directed():

        def _sort_source_target(edge):
            source, target, edge_data = edge
            if source > target:
                temp = source
                source = target
                target = temp
            return source, target, edge_data

        edges = [_sort_source_target(edge) for edge in edges]

    def _get_edge_key(source: Any, target: Any) -> str:
        return f"{source} -> {target}"

    edges = sorted(edges, key=lambda x: _get_edge_key(x[0], x[1]))

    fixed_graph.add_edges_from(edges)
    return fixed_graph


def stable_largest_connected_component(graph: nx.Graph) -> nx.Graph:
    """Return the largest connected component of the graph, with nodes and edges sorted in a stable way."""
    # NOTE: The import is done here to reduce the initial import time of the module

    graph = graph.copy()
    graph = cast("nx.Graph", largest_connected_component(graph))
    graph = normalize_node_names(graph)
    return _stabilize_graph(graph)


def main():
    # 1. 解析命令行参数
    parser = argparse.ArgumentParser()
    parser.add_argument('--input', required=True, help='json string')
    args = parser.parse_args()
    # 2. 读取JSON到DataFrame<sup>2</sup>
    try:
        edges_df = pd.read_json(args.input, orient='records')
    except Exception as e:
        print(f"Error reading JSON: {str(e)}")
        return
    # 3. 构建图结构
    graph = nx.from_pandas_edgelist(
        edges_df,
    )
    graph = stable_largest_connected_component(graph)
    # 4. 执行社区检测
    community_mapping = hierarchical_leiden(
        graph,
        max_cluster_size=10,
        random_seed=0xDEADBEEF
    )

    def cluster_to_dict(cluster):
        data = cluster._asdict()  # 将NamedTuple转为字典<sup>6</sup>
        if data["parent_cluster"] is None:
            data["parent_cluster"] = -1  # 替换null为-1
        return data

    community_list = [cluster_to_dict(c) for c in community_mapping]

    print(json.dumps(community_list))


if __name__ == "__main__":
    main()
