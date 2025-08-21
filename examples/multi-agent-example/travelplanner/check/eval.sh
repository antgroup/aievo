#!/bin/bash

# 集成评估脚本启动器
# 使用示例: ./eval.sh ../../output/train_v0_20250819203412.json

set -e

INPUT_FILE=../output/72-validation_t0_20250821112215.json

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
