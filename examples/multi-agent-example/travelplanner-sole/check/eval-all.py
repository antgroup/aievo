#!/usr/bin/env python3
"""
集成评估脚本 - 合并四个评估步骤为一个文件
包含：parsing, element_extraction, combination, evaluation
"""

import os
import sys
import json
import argparse
import time
from tqdm import tqdm
from datetime import datetime
from typing import List, Dict, Any
import traceback

# 添加路径以导入必要模块
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "../postprocess")))
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "../evaluation")))

# 导入必要的模块
try:
    from datasets import load_dataset
    from postprocess.openai_request import build_plan_format_conversion_prompt, prompt_chatgpt
    from evaluation.commonsense_constraint import evaluation as commonsense_eval
    from evaluation.hard_constraint import evaluation as hard_eval
except ImportError as e:
    print(f"Warning: Import error: {e}")
    print("Some functionality may not be available")

class IntegratedEvaluator:
    """集成评估器 - 执行完整的评估流程"""
    
    def __init__(self, args):
        self.args = args
        self.query_data_list = None
        self.parsed_results = []
        self.extracted_results = []
        self.submission_list = []
        
        # 测试模式限制
        if args.test_mode:
            self.limit = 5
            print("🧪 测试模式：只处理前5个计划")
        elif args.limit:
            self.limit = args.limit
            print(f"🔢 限制模式：只处理前{self.limit}个计划")
        else:
            self.limit = None
        
        # 设置路径
        self.setup_paths()
        
    def setup_paths(self):
        """设置所有必要的路径"""
        # 基础路径
        self.output_dir = self.args.output_dir
        self.dataset_dir = f'{self.output_dir}/datasets/{self.args.set_type}'
        self.dataset_file = os.path.join(self.dataset_dir, f'travelplanner_{self.args.set_type}_dataset.json')
        
        # 生成文件路径
        self.generation_dir = f'{self.output_dir}/{self.args.set_type}'
        
        # 临时文件路径
        self.tmp_dir = self.args.tmp_dir or f'{self.output_dir}/parse'
        os.makedirs(self.tmp_dir, exist_ok=True)
        
        # 提交文件路径
        self.submission_dir = self.args.submission_file_dir or f'{self.output_dir}/eval'
        os.makedirs(self.submission_dir, exist_ok=True)
        
        # 结果文件路径
        if self.args.mode == 'two-stage':
            suffix = ''
        elif self.args.mode == 'sole-planning':
            suffix = f'_{self.args.strategy}'
        
        self.submission_file = f'{self.submission_dir}/{self.args.set_type}_{self.args.model_name}{suffix}_{self.args.mode}_submission.jsonl'
        
        print(f"输出目录: {self.output_dir}")
        print(f"数据集文件: {self.dataset_file}")
        print(f"生成计划目录: {self.generation_dir}")
        print(f"最终提交文件: {self.submission_file}")
        
    def load_dataset(self):
        """加载数据集"""
        print("📁 加载数据集...")
        
        # 检查本地数据集文件
        if os.path.exists(self.dataset_file):
            print(f"从本地文件加载数据集: {self.dataset_file}")
            try:
                with open(self.dataset_file, 'r', encoding='utf-8') as f:
                    self.query_data_list = json.load(f)
                print(f"成功加载 {len(self.query_data_list)} 个数据项")
                
                # 应用限制
                if self.limit:
                    original_length = len(self.query_data_list)
                    self.query_data_list = self.query_data_list[:self.limit]
                    print(f"应用限制：从 {original_length} 个减少到 {len(self.query_data_list)} 个")
                
                return
            except Exception as e:
                print(f"加载本地数据集失败: {e}")
                print("回退到从 HuggingFace 下载...")
        
        # 从 HuggingFace 下载
        print(f"从 HuggingFace 下载数据集: osunlp/TravelPlanner [{self.args.set_type}]")
        try:
            if self.args.set_type == 'validation':
                hf_dataset = load_dataset('osunlp/TravelPlanner', 'validation')['validation']
            elif self.args.set_type == 'test':
                hf_dataset = load_dataset('osunlp/TravelPlanner', 'test')['test']
            elif self.args.set_type == 'train':
                hf_dataset = load_dataset('osunlp/TravelPlanner', 'train')['train']
            else:
                raise ValueError(f"不支持的数据集类型: {self.args.set_type}")
            
            # 转换为列表格式
            self.query_data_list = [dict(item) for item in hf_dataset]
            print(f"从 HuggingFace 下载 {len(self.query_data_list)} 个数据项")
            
            # 应用限制
            if self.limit:
                original_length = len(self.query_data_list)
                self.query_data_list = self.query_data_list[:self.limit]
                print(f"应用限制：从 {original_length} 个减少到 {len(self.query_data_list)} 个")
            
            # 保存到本地以备后用
            os.makedirs(self.dataset_dir, exist_ok=True)
            with open(self.dataset_file, 'w', encoding='utf-8') as f:
                json.dump(self.query_data_list, f, indent=2, ensure_ascii=False)
            print(f"数据集已保存到本地: {self.dataset_file}")
            
        except Exception as e:
            print(f"从 HuggingFace 下载数据集失败: {e}")
            raise
    
    def step1_parsing(self):
        """步骤1: 解析生成的计划文本"""
        print("\n🔄 步骤 1/4: 解析计划文本...")
        
        try:
            # 构建解析提示词
            prompt_list = build_plan_format_conversion_prompt(
                directory=self.output_dir,
                set_type=self.args.set_type,
                model_name=self.args.model_name,
                strategy=self.args.strategy,
                mode=self.args.mode
            )
            
            # 准备输出文件
            if self.args.mode == 'two-stage':
                suffix = ''
            elif self.args.mode == 'sole-planning':
                suffix = f'_{self.args.strategy}'
            
            tmp_output_file = f'{self.tmp_dir}/{self.args.set_type}_{self.args.model_name}{suffix}_{self.args.mode}.txt'
            
            total_price = 0
            self.parsed_results = []
            
            print(f"处理 {len(prompt_list)} 个计划...")
            for idx, prompt in enumerate(tqdm(prompt_list)):
                if prompt == "":
                    result = str(idx)
                    self.parsed_results.append(result)
                    with open(tmp_output_file, 'a+', encoding='utf-8') as f:
                        f.write(result + '\n')
                    continue
                
                try:
                    # 调用 LLM 进行解析
                    env_model_name = "Qwen2.5-72B-Instruct"
                    results, _, price = prompt_chatgpt(
                        "You are a helpful assistant.", 
                        index=idx, 
                        save_path=tmp_output_file,
                        user_input=prompt, 
                        model_name=env_model_name, 
                        temperature=0
                    )
                    total_price += price
                    self.parsed_results.append(results)
                    
                except Exception as e:
                    print(f"解析第 {idx} 个计划时出错: {e}")
                    error_result = f"ERROR: {str(e)}"
                    self.parsed_results.append(error_result)
                    with open(tmp_output_file, 'a+', encoding='utf-8') as f:
                        f.write(f"{idx}\t{error_result}\n")
            
            print(f"解析完成，总费用: ${total_price}")
            return True
            
        except Exception as e:
            print(f"解析步骤失败: {e}")
            traceback.print_exc()
            return False
    
    def step2_element_extraction(self):
        """步骤2: 提取结构化数据"""
        print("\n🎯 步骤 2/4: 提取结构化数据...")
        
        try:
            # 读取解析结果
            if self.args.mode == 'two-stage':
                suffix = ''
            elif self.args.mode == 'sole-planning':
                suffix = f'_{self.args.strategy}'
            
            tmp_file = f'{self.tmp_dir}/{self.args.set_type}_{self.args.model_name}{suffix}_{self.args.mode}.txt'
            
            if not os.path.exists(tmp_file):
                print(f"临时文件不存在: {tmp_file}")
                return False
            
            with open(tmp_file, 'r', encoding='utf-8') as f:
                results = f.read().strip().split('\n')
            
            idx_number_list = [i for i in range(1, len(self.query_data_list) + 1)]
            
            print(f"处理 {len(idx_number_list)} 个提取任务...")
            success_count = 0
            
            for idx in tqdm(idx_number_list):
                try:
                    # 加载生成的计划
                    plan_file = f'{self.generation_dir}/generated_plan_{idx}.json'
                    if not os.path.exists(plan_file):
                        print(f"计划文件不存在: {plan_file}")
                        continue
                    
                    with open(plan_file, 'r', encoding='utf-8') as f:
                        generated_plan = json.load(f)
                    
                    # 检查是否有有效结果
                    plan_key = f'{self.args.model_name}{suffix}_{self.args.mode}_results'
                    if generated_plan[-1][plan_key] not in ["", "Max Token Length Exceeded."]:
                        try:
                            # 提取 JSON 内容
                            result_text = results[idx-1]
                            if '```json' in result_text:
                                result = result_text.split('```json')[1].split('```')[0]
                            else:
                                result = result_text
                            
                            # 解析 JSON
                            parsed_result = eval(result.strip())
                            
                            # 保存解析结果
                            parsed_key = f'{self.args.model_name}{suffix}_{self.args.mode}_parsed_results'
                            generated_plan[-1][parsed_key] = parsed_result
                            
                            success_count += 1
                            
                        except Exception as e:
                            print(f"解析第 {idx} 个结果时出错: {e}")
                            # 设置为 None 表示解析失败
                            parsed_key = f'{self.args.model_name}{suffix}_{self.args.mode}_parsed_results'
                            generated_plan[-1][parsed_key] = None
                    else:
                        # 没有有效结果
                        parsed_key = f'{self.args.model_name}{suffix}_{self.args.mode}_parsed_results'
                        generated_plan[-1][parsed_key] = None
                    
                    # 保存更新的计划文件
                    with open(plan_file, 'w', encoding='utf-8') as f:
                        json.dump(generated_plan, f, indent=2, ensure_ascii=False)
                        
                except Exception as e:
                    print(f"处理第 {idx} 个文件时出错: {e}")
                    continue
            
            print(f"提取完成，成功处理: {success_count}/{len(idx_number_list)}")
            return True
            
        except Exception as e:
            print(f"提取步骤失败: {e}")
            traceback.print_exc()
            return False
    
    def step3_combination(self):
        """步骤3: 合并为评估格式"""
        print("\n📊 步骤 3/4: 合并文件...")
        
        try:
            if self.args.mode == 'two-stage':
                suffix = ''
            elif self.args.mode == 'sole-planning':
                suffix = f'_{self.args.strategy}'
            
            idx_number_list = [i for i in range(1, len(self.query_data_list) + 1)]
            self.submission_list = []
            
            print(f"合并 {len(idx_number_list)} 个计划...")
            for idx in tqdm(idx_number_list):
                try:
                    plan_file = f'{self.generation_dir}/generated_plan_{idx}.json'
                    if not os.path.exists(plan_file):
                        # 如果文件不存在，创建空计划
                        plan = None
                    else:
                        with open(plan_file, 'r', encoding='utf-8') as f:
                            generated_plan = json.load(f)
                        
                        parsed_key = f'{self.args.model_name}{suffix}_{self.args.mode}_parsed_results'
                        plan = generated_plan[-1].get(parsed_key, None)
                    
                    # 添加到提交列表
                    submission_item = {
                        "idx": idx,
                        "query": self.query_data_list[idx-1]['query'],
                        "plan": plan
                    }
                    self.submission_list.append(submission_item)
                    
                except Exception as e:
                    print(f"处理第 {idx} 个计划时出错: {e}")
                    # 添加空计划
                    submission_item = {
                        "idx": idx,
                        "query": self.query_data_list[idx-1]['query'],
                        "plan": None
                    }
                    self.submission_list.append(submission_item)
            
            # 写入提交文件（第一次写入操作）
            print(f"💾 写入提交文件: {self.submission_file}")
            with open(self.submission_file, 'w', encoding='utf-8') as w:
                for unit in self.submission_list:
                    output = json.dumps(unit, ensure_ascii=False)
                    w.write(output + "\n")
            
            print(f"合并完成，生成 {len(self.submission_list)} 个提交项")
            return True
            
        except Exception as e:
            print(f"合并步骤失败: {e}")
            traceback.print_exc()
            return False
    
    def step4_evaluation(self):
        """步骤4: 评估结果"""
        print("\n📈 步骤 4/4: 评估结果...")
        
        try:
            # 使用现有的评估函数
            scores, detailed_scores = self.eval_score(self.args.set_type, self.submission_file)
            
            # 输出结果
            print("\n" + "="*60)
            print("🎯 评估结果:")
            print("="*60)
            
            for key in scores:
                percentage = float(int(scores[key]*10000))/100
                print(f"{key}: {percentage}%")
            
            print("\n" + "="*60)
            print("📊 详细结果:")
            print("="*60)
            print(json.dumps(detailed_scores, indent=2, ensure_ascii=False))
            
            # 保存结果到文件（第二次写入操作）
            self.save_results(scores, detailed_scores)
            
            return True, scores, detailed_scores
            
        except Exception as e:
            print(f"评估步骤失败: {e}")
            traceback.print_exc()
            return False, None, None
    
    def eval_score(self, set_type: str, file_path: str):
        """评估分数 - 从原 eval.py 移植的逻辑"""
        
        # 加载测试计划
        tested_plans = self.load_line_json_data(file_path)
        
        # 初始化统计变量
        hardConstraint_statistic = {level: {day: [] for day in [3,5,7]} for level in ['easy','medium','hard']}
        commonsenseConstraint_statistic = {level: {day: [] for day in [3,5,7]} for level in ['easy','medium','hard']}
        
        delivery_cnt = 0
        plan_constraint_store = []
        
        print(f"评估 {min(len(self.query_data_list), len(tested_plans))} 个计划...")
        
        for idx in tqdm(range(0, min(len(self.query_data_list), len(tested_plans)))):
            query_data = self.query_data_list[idx]
            tested_plan = tested_plans[idx]
            
            # 数据类型转换
            if type(query_data) == str:
                query_data = eval(query_data)
            if type(tested_plan) == str:
                tested_plan = eval(tested_plan)
            if type(query_data['local_constraint']) == str:
                query_data['local_constraint'] = eval(query_data['local_constraint'])
            
            # 常识约束评估
            if tested_plan['plan']:
                delivery_cnt += 1
                commonsense_info_box = commonsense_eval(query_data, tested_plan['plan'])
            else:
                commonsense_info_box = None
            
            # 硬约束评估（只有通过常识约束才评估）
            if commonsense_info_box and commonsense_info_box['is_not_absent'][0] and commonsense_info_box['is_valid_information_in_sandbox'][0]:
                hard_info_box = hard_eval(query_data, tested_plan['plan'])
            else:
                hard_info_box = None
            
            plan_constraint_store.append({
                'commonsense_constraint': commonsense_info_box,
                'hard_constraint': hard_info_box
            })
            
            commonsenseConstraint_statistic[query_data['level']][query_data['days']].append(commonsense_info_box)
            hardConstraint_statistic[query_data['level']][query_data['days']].append(hard_info_box)
        
        # 处理约束统计
        constraint_record = {key: {day: {'house rule':0, 'cuisine':0, 'room type':0, 'transportation':0} for day in [3,5,7]} for key in ['medium','hard']}
        constraint_mapping = {'house rule':'valid_room_rule','cuisine':'valid_cuisine','room type':'valid_room_type','transportation':'valid_transportation'}
        mapping_constraint_record = {key: {day: {'valid_room_rule':0, 'valid_cuisine':0, 'valid_room_type':0, 'valid_transportation':0} for day in [3,5,7]} for key in ['medium','hard']}
        count_record = {key:{day:0 for day in [3,5,7]} for key in ['easy','medium','hard']}
        
        for unit in self.query_data_list:
            if type(unit) == str:
                unit = eval(unit)
            if type(unit['local_constraint']) == str:
                unit['local_constraint'] = eval(unit['local_constraint'])
                
            count_record[unit['level']][unit['days']] += 1
            for key in constraint_record['medium'][3]:
                try:
                    if unit['local_constraint'][key] != None:
                        if unit['level'] in constraint_record:  # 只处理 medium 和 hard 级别
                            constraint_record[unit['level']][unit['days']][key] += 1
                            mapping_constraint_record[unit['level']][unit['days']][constraint_mapping[key]] += 1
                except Exception:
                    continue
        
        # 统计处理
        commonsenseConstraint_statistic_processed = self.statistics(commonsenseConstraint_statistic)
        hardConstraint_statistic_processed = self.statistics(hardConstraint_statistic)
        
        # 计算最终分数
        final_all_cnt = 0
        final_commonsense_cnt = 0
        final_hardConstraint_cnt = 0
        
        constraint_dis_record = {"commonsense":{"pass":0,"total":0},"hard":{"pass":0,"total":0}}
        
        # 详细的约束统计处理（简化版本）
        for constraint in ['commonsense','hard']:
            if constraint == 'commonsense':
                constraint_statistic = commonsenseConstraint_statistic_processed
                key_list = ['is_valid_information_in_current_city','is_valid_information_in_sandbox','is_reasonalbe_visiting_city','is_valid_restaurants','is_valid_transportation','is_valid_attractions','is_valid_accommodation','is_not_absent']
            else:
                constraint_statistic = hardConstraint_statistic_processed
                key_list = ['valid_cost','valid_room_rule','valid_cuisine','valid_room_type','valid_transportation']
            
            for level in constraint_statistic:
                for day in constraint_statistic[level]:
                    for key in key_list:
                        if key in constraint_statistic[level][day]:
                            constraint_dis_record[constraint]['pass'] += constraint_statistic[level][day][key]['true']
                            if constraint == 'commonsense':
                                constraint_dis_record[constraint]['total'] += count_record[level][day]
                            else:
                                if key in ['valid_room_rule','valid_cuisine','valid_room_type','valid_transportation']:
                                    # 只有 medium 和 hard 级别有 mapping_constraint_record
                                    if level in ['medium', 'hard']:
                                        constraint_dis_record[constraint]['total'] += mapping_constraint_record[level][day].get(key, 0)
                                    # easy 级别直接使用 count_record
                                    else:
                                        constraint_dis_record[constraint]['total'] += count_record[level][day]
                                else:
                                    constraint_dis_record[constraint]['total'] += count_record[level][day]
        
        # 计算宏观通过率
        for idx in range(0, min(len(self.query_data_list), len(plan_constraint_store))):
            if plan_constraint_store[idx]['commonsense_constraint']:
                final_commonsense_pass = True
                final_hardConstraint_pass = True
                
                # 检查常识约束
                for item in plan_constraint_store[idx]['commonsense_constraint']:
                    if plan_constraint_store[idx]['commonsense_constraint'][item][0] is not None and not plan_constraint_store[idx]['commonsense_constraint'][item][0]:
                        final_commonsense_pass = False
                        break
                
                # 检查硬约束
                if plan_constraint_store[idx]['hard_constraint'] is None:
                    continue
                    
                for item in plan_constraint_store[idx]['hard_constraint']:
                    if plan_constraint_store[idx]['hard_constraint'][item][0] is not None and plan_constraint_store[idx]['hard_constraint'][item][0] == False:
                        final_hardConstraint_pass = False
                        break
                
                if final_commonsense_pass:
                    final_commonsense_cnt += 1
                if final_hardConstraint_pass:
                    final_hardConstraint_cnt += 1
                if final_commonsense_pass and final_hardConstraint_pass:
                    final_all_cnt += 1
        
        # 计算最终结果
        result = {}
        
        # 根据数据集类型设置总数
        if set_type == 'train':
            total_count = 45
            commonsense_total = 360
            hard_total = 105
        elif set_type == 'validation':
            total_count = 180
            commonsense_total = 1440
            hard_total = 420
        elif set_type == 'test':
            total_count = 1000
            commonsense_total = 8000
            hard_total = 2290
        else:
            # 动态计算
            total_count = min(len(self.query_data_list), len(tested_plans))
            commonsense_total = constraint_dis_record['commonsense']['total'] or total_count * 8
            hard_total = constraint_dis_record['hard']['total'] or 1
        
        result['Delivery Rate'] = delivery_cnt / total_count if total_count > 0 else 0
        result['Commonsense Constraint Micro Pass Rate'] = constraint_dis_record['commonsense']['pass'] / commonsense_total if commonsense_total > 0 else 0
        result['Commonsense Constraint Macro Pass Rate'] = final_commonsense_cnt / total_count if total_count > 0 else 0
        result['Hard Constraint Micro Pass Rate'] = constraint_dis_record['hard']['pass'] / hard_total if hard_total > 0 else 0
        result['Hard Constraint Macro Pass Rate'] = final_hardConstraint_cnt / total_count if total_count > 0 else 0
        result['Final Pass Rate'] = final_all_cnt / total_count if total_count > 0 else 0
        
        # 详细结果
        remap_commonsense_constraint_record, remap_hard_constraint_record = self.paper_term_mapping(
            commonsenseConstraint_statistic_processed, hardConstraint_statistic_processed)
        
        detailed_scores = {
            "Commonsense Constraint": remap_commonsense_constraint_record,
            "Hard Constraint": remap_hard_constraint_record
        }
        
        return result, detailed_scores
    
    def statistics(self, constraint_statistic):
        """统计约束结果"""
        result = {level: {day: {} for day in constraint_statistic[level]} for level in constraint_statistic}
        
        for level, days in constraint_statistic.items():
            for day, dicts in days.items():
                for dct in dicts:
                    if dct:
                        for key, data in dct.items():
                            true_count = data.count(True) if isinstance(data, list) else (1 if data[0] else 0)
                            false_count = data.count(False) if isinstance(data, list) else (0 if data[0] else 1)
                            if key not in result[level][day]:
                                result[level][day][key] = {"true": 0, "false": 0}
                            result[level][day][key]["true"] += true_count
                            result[level][day][key]["false"] += false_count
        
        return result
    
    def paper_term_mapping(self, commonsense_constraint_record, hard_constraint_record):
        """映射论文术语"""
        mapping_dict = {
            'is_valid_information_in_current_city': 'Within Current City',
            'is_valid_information_in_sandbox': 'Within Sandbox',
            'is_reasonalbe_visiting_city': 'Reasonable City Route',
            'is_valid_restaurants': 'Diverse Restaurants',
            'is_valid_transportation': 'Non-conf. Transportation',
            'is_valid_attractions': 'Diverse Attractions',
            'is_valid_accommodation': 'Minimum Nights Stay',
            'is_not_absent': 'Complete Information',
            'valid_cost': 'Budget',
            'valid_room_rule': 'Room Rule',
            'valid_cuisine': 'Cuisine',
            'valid_room_type': 'Room Type',
            'valid_transportation': 'Transportation'
        }
        
        remap_commonsense = {level: {day: {} for day in [3,5,7]} for level in ['easy','medium','hard']}
        remap_hard = {level: {day: {} for day in [3,5,7]} for level in ['easy','medium','hard']}
        
        for level in commonsense_constraint_record:
            for day in commonsense_constraint_record[level]:
                remap_commonsense[level][day] = {
                    mapping_dict.get(key, key): val 
                    for key, val in commonsense_constraint_record[level][day].items()
                }
                remap_hard[level][day] = {
                    mapping_dict.get(key, key): val 
                    for key, val in hard_constraint_record[level][day].items()
                }
        
        return remap_commonsense, remap_hard
    
    def load_line_json_data(self, filename):
        """加载 JSONL 文件"""
        data = []
        with open(filename, 'r', encoding='utf-8') as f:
            for line in f.read().strip().split('\n'):
                if line.strip():
                    unit = json.loads(line)
                    data.append(unit)
        return data
    
    def save_results(self, scores, detailed_scores):
        """保存评估结果"""
        print("\n💾 保存评估结果...")
        
        # 从提交文件路径生成结果文件名
        base_name = os.path.basename(self.submission_file)
        if base_name.endswith('.jsonl'):
            base_name = base_name[:-6]  # 移除.jsonl后缀
        
        # 创建结果目录
        result_dir = os.path.join(os.path.dirname(self.submission_file), 'results')
        os.makedirs(result_dir, exist_ok=True)
        
        # 生成时间戳
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        
        # 格式化分数结果为百分比
        formatted_scores = {}
        for key in scores:
            formatted_scores[key] = f"{float(int(scores[key]*10000))/100}%"
        
        # 保存合并结果
        combined_results = {
            'scores': formatted_scores,
            'detailed_scores': detailed_scores,
            'evaluation_file': self.submission_file,
            'set_type': self.args.set_type,
            'model_name': self.args.model_name,
            'strategy': self.args.strategy,
            'mode': self.args.mode,
            'timestamp': timestamp
        }
        
        combined_filename = f"{base_name}_results_{timestamp}.json"
        combined_path = os.path.join(result_dir, combined_filename)
        
        with open(combined_path, 'w', encoding='utf-8') as f:
            json.dump(combined_results, f, indent=2, ensure_ascii=False)
        
        print(f"📁 结果已保存到: {combined_path}")
        return combined_path
    
    def run(self):
        """执行完整的评估流程"""
        print("🚀 开始集成评估流程...")
        print("="*60)
        
        start_time = time.time()
        
        try:
            # 加载数据集
            self.load_dataset()
            
            # 步骤1: 解析
            if not self.step1_parsing():
                print("❌ 步骤1失败，停止执行")
                return False
            
            # 步骤2: 提取
            if not self.step2_element_extraction():
                print("❌ 步骤2失败，停止执行")
                return False
            
            # 步骤3: 合并（第一次写入）
            if not self.step3_combination():
                print("❌ 步骤3失败，停止执行")
                return False
            
            # 步骤4: 评估（第二次写入）
            success, scores, detailed_scores = self.step4_evaluation()
            if not success:
                print("❌ 步骤4失败，停止执行")
                return False
            
            end_time = time.time()
            total_time = end_time - start_time
            
            print("\n" + "="*60)
            print("✅ 集成评估完成!")
            print(f"⏱️  总耗时: {total_time:.2f} 秒")
            print("="*60)
            
            return True
            
        except Exception as e:
            print(f"\n❌ 评估过程中出现错误: {e}")
            traceback.print_exc()
            return False


def main():
    """主函数"""
    parser = argparse.ArgumentParser(description="集成评估脚本")
    parser.add_argument("--set_type", type=str, default="validation", choices=["train", "validation", "test"],
                       help="数据集类型")
    parser.add_argument("--model_name", type=str, default="Qwen2.5-72B-Instruct",
                       help="模型名称")
    parser.add_argument("--strategy", type=str, default="direct", 
                       choices=["direct", "cot", "react", "evoagent"],
                       help="策略名称")
    parser.add_argument("--mode", type=str, default="sole-planning", 
                       choices=["two-stage", "sole-planning"],
                       help="模式")
    parser.add_argument("--output_dir", type=str, default="/Users/liuxiansheng/Agent/myevoagent/output",
                       help="输出目录")
    parser.add_argument("--tmp_dir", type=str, default=None,
                       help="临时文件目录")
    parser.add_argument("--submission_file_dir", type=str, default=None,
                       help="提交文件目录")
    parser.add_argument("--test_mode", action="store_true",
                       help="测试模式：只处理前5个计划")
    parser.add_argument("--limit", type=int, default=None,
                       help="限制处理的计划数量")
    
    args = parser.parse_args()
    
    print("📋 评估配置:")
    print(f"  数据集类型: {args.set_type}")
    print(f"  模型名称: {args.model_name}")
    print(f"  策略: {args.strategy}")
    print(f"  模式: {args.mode}")
    print(f"  输出目录: {args.output_dir}")
    print()
    
    # 创建评估器并运行
    evaluator = IntegratedEvaluator(args)
    success = evaluator.run()
    
    if success:
        print("🎉 评估成功完成!")
        return 0
    else:
        print("💥 评估失败!")
        return 1


if __name__ == "__main__":
    exit(main())
