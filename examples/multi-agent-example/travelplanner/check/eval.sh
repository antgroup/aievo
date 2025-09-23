#!/bin/bash

set -e

INPUT_FILE=../output/eval_rep3_tan_wgr262_20250922214654.json
# INPUT_FILE=../../travelplanner-sole/output/train_v3_20250918193921.json


# 路径配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🚀 启动集成评估流程"
echo "================================"
echo "输入文件: $INPUT_FILE"
echo "================================"

# 运行集成评估
echo "🔄 开始评估..."
python3 "$SCRIPT_DIR/eval.py" "$INPUT_FILE"

echo ""
echo "✅ 评估完成!"
