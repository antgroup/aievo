#!/usr/bin/env python3
"""
从travelplanner训练集中随机选择20个样本作为新的训练集，其余25个作为评估集，保持选中样本的相对顺序并添加ID
"""

import json
import random
import os
from pathlib import Path

def split_dataset(input_file, train_size=20, seed=4):
    """
    分割数据集，随机选择样本但保持相对顺序，并为每个样本添加ID
    
    Args:
        input_file: 输入文件路径
        train_size: 训练集大小，默认20
        seed: 随机种子，保证结果可重复
    """
    # 设置随机种子
    random.seed(seed)
    
    # 读取原始数据
    with open(input_file, 'r', encoding='utf-8') as f:
        data = json.load(f)
    
    print(f"原始数据集包含 {len(data)} 个样本")
    
    # 为每个样本添加原始索引
    for i, sample in enumerate(data):
        sample['original_index'] = i
    
    # 随机选择训练集的索引，但保持这些索引的相对顺序
    total_indices = list(range(len(data)))
    train_indices = sorted(random.sample(total_indices, train_size))
    eval_indices = sorted([i for i in total_indices if i not in train_indices])
    
    print(f"选中的训练集索引: {train_indices[:5]}...") # 只显示前5个
    print(f"选中的评估集索引: {eval_indices[:5]}...") # 只显示前5个
    
    # 根据选中的索引创建训练集和评估集（保持相对顺序）
    train_data = [data[i] for i in train_indices]
    eval_data = [data[i] for i in eval_indices]
    
    # 为训练集和评估集样本分别添加ID
    for i, sample in enumerate(train_data):
        sample['id'] = i + 1
    
    for i, sample in enumerate(eval_data):
        sample['id'] = i + 1
    
    print(f"训练集: {len(train_data)} 个样本")
    print(f"评估集: {len(eval_data)} 个样本")
    
    # 获取输入文件的目录
    input_dir = Path(input_file).parent
    
    # 保存训练集
    train_file = input_dir / "travelplanner_train_split.json"
    with open(train_file, 'w', encoding='utf-8') as f:
        json.dump(train_data, f, ensure_ascii=False, indent=2)
    print(f"训练集保存到: {train_file}")
    
    # 保存评估集
    eval_file = input_dir / "travelplanner_eval_split.json"
    with open(eval_file, 'w', encoding='utf-8') as f:
        json.dump(eval_data, f, ensure_ascii=False, indent=2)
    print(f"评估集保存到: {eval_file}")
    
    return train_file, eval_file

def main():
    # 获取当前脚本所在目录
    current_dir = Path(__file__).parent
    input_file = current_dir / "travelplanner_train_dataset.json"
    
    if not input_file.exists():
        print(f"错误: 找不到输入文件 {input_file}")
        return
    
    print(f"处理文件: {input_file}")
    train_file, eval_file = split_dataset(input_file)
    
    # 验证分割结果
    print("\n验证分割结果:")
    with open(train_file, 'r', encoding='utf-8') as f:
        train_data = json.load(f)
    with open(eval_file, 'r', encoding='utf-8') as f:
        eval_data = json.load(f)
    
    print(f"训练集样本数: {len(train_data)}")
    print(f"评估集样本数: {len(eval_data)}")
    print(f"总样本数: {len(train_data) + len(eval_data)}")
    
    # 显示一些样本的基本信息
    print("\n训练集样本示例:")
    for i, sample in enumerate(train_data[:3]):
        print(f"  样本ID {sample['id']} (原索引{sample['original_index']}): {sample['org']} -> {sample['dest']}, {sample['days']}天, 难度: {sample['level']}")
    
    print("\n评估集样本示例:")
    for i, sample in enumerate(eval_data[:3]):
        print(f"  样本ID {sample['id']} (原索引{sample['original_index']}): {sample['org']} -> {sample['dest']}, {sample['days']}天, 难度: {sample['level']}")

if __name__ == "__main__":
    main()
