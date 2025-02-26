import argparse
import json
import networkx as nx
import pandas as pd
import numpy as np
from graspologic.partition import hierarchical_leiden

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
        source='source',  # 根据实际列名调整
        target='target'   # 根据实际列名调整
    )
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
