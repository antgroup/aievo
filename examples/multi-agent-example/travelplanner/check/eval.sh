#!/bin/bash

set -e

INPUT_FILE=../output/train_rev3.1.1.1_t01_20250904103255.json

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
