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


# 导入必要的模块
try:
    from datasets import load_dataset
    from openai_request import build_plan_format_conversion_prompt, prompt_chatgpt
    from commonsense_constraint import evaluation as commonsense_eval
    from hard_constraint import evaluation as hard_eval
except ImportError as e:
    print(f"Warning: Import error: {e}")
    print("Some functionality may not be available")


class IntegratedEvaluator:
    """集成评估器 - 执行完整的评估流程"""
    
    def __init__(self, args):
        self.args = args
        self.query_data_list = None  # 真正的查询数据从数据集中加载
        self.model_outputs = None    # 模型输出数据从输入文件中加载
        self.parsed_results = []
        
        # 从输入文件中加载模型输出数据
        self.load_model_outputs()
        
        # 从数据集中加载真正的查询数据
        self.load_dataset()
        
        # 设置路径
        self.setup_paths()
        
    def setup_paths(self):
        """设置所有必要的路径"""
        # 基础路径
        input_dir = os.path.dirname(self.args.input_file)
        self.results_dir = '../results'
        os.makedirs(self.results_dir, exist_ok=True)
        
        # 临时文件路径
        self.tmp_dir = self.results_dir + '/parse'
        os.makedirs(self.tmp_dir, exist_ok=True)
        
        # 提交文件路径
        self.submission_dir = self.results_dir + '/sub'
        os.makedirs(self.submission_dir, exist_ok=True)
        
        # 从输入文件名派生输出文件名
        base_name = os.path.basename(self.args.input_file)
        if base_name.endswith('.json'):
            base_name = base_name[:-5]
        
        self.submission_file = os.path.join(self.submission_dir, f'{base_name}_submission.jsonl')
        
        print(f"输入文件: {self.args.input_file}")
        print(f"结果目录: {self.results_dir}")
        print(f"最终提交文件: {self.submission_file}")

    def load_model_outputs(self):
        """从单个输入文件加载模型输出数据"""
        print(f"📁 从输入文件加载模型输出数据: {self.args.input_file}")
        try:
            with open(self.args.input_file, 'r', encoding='utf-8') as f:
                self.model_outputs = json.load(f)
            print(f"成功加载 {len(self.model_outputs)} 个模型输出")
        except Exception as e:
            print(f"加载输入文件失败: {e}")
            raise

    def load_dataset(self):
        """从数据集中加载真正的查询数据"""
        print("📁 从数据集加载查询数据...")
        
        # 确定数据集类型
        if 'train' in self.args.input_file:
            set_type = 'train'
        elif 'validation' in self.args.input_file:
            set_type = 'validation'
        elif 'test' in self.args.input_file:
            set_type = 'test'
        else:
            print("警告: 无法从文件名确定数据集类型。默认为 'validation'。")
            set_type = 'validation'
            
        self.set_type = set_type
        print(f"推断的数据集类型: {self.set_type}")
        
        # 构建数据集文件路径
        self.dataset_file = f'../../../../dataset/travelplanner/{set_type}/travelplanner_{set_type}_dataset.json'
        
        try:
            with open(self.dataset_file, 'r', encoding='utf-8') as f:
                self.query_data_list = json.load(f)
            print(f"成功加载 {len(self.query_data_list)} 个数据项")
        except Exception as e:
            print(f"加载数据集文件失败: {e}")
            # 如果本地文件不存在，尝试从 HuggingFace 下载
            print("尝试从 HuggingFace 下载数据集...")
            try:
                if set_type == 'validation':
                    hf_dataset = load_dataset('osunlp/TravelPlanner', 'validation')['validation']
                elif set_type == 'test':
                    hf_dataset = load_dataset('osunlp/TravelPlanner', 'test')['test']
                elif set_type == 'train':
                    hf_dataset = load_dataset('osunlp/TravelPlanner', 'train')['train']
                
                # 转换为列表格式
                self.query_data_list = [dict(item) for item in hf_dataset]
                print(f"从 HuggingFace 下载 {len(self.query_data_list)} 个数据项")
                
                # 保存到本地以备后用
                os.makedirs(os.path.dirname(self.dataset_file), exist_ok=True)
                with open(self.dataset_file, 'w', encoding='utf-8') as f:
                    json.dump(self.query_data_list, f, indent=2, ensure_ascii=False)
                print(f"数据集已保存到本地: {self.dataset_file}")
                
            except Exception as e2:
                print(f"从 HuggingFace 下载数据集也失败: {e2}")
                raise
    
    def step1_parsing(self):
        """步骤1: 解析生成的计划文本"""
        print("\n🔄 步骤 1/4: 解析计划文本...")
        
        # 准备输出文件路径
        base_name = os.path.basename(self.args.input_file).replace('.json', '')
        tmp_output_file = os.path.join(self.tmp_dir, f'{base_name}_parsed.txt')
        
        # 检查是否已经完成解析
        if os.path.exists(tmp_output_file) and os.path.getsize(tmp_output_file) > 0:
            print(f"✅ 解析文件已存在且非空: {tmp_output_file}")
            print("跳过步骤1，直接加载已有的解析结果...")
            
            # 加载已有的解析结果
            try:
                with open(tmp_output_file, 'r', encoding='utf-8') as f:
                    lines = f.read().strip().split('\n')
                    self.parsed_results = []
                    for line in lines:
                        if '\t' in line:
                            # 格式: idx\tresult
                            parts = line.split('\t', 1)
                            if len(parts) == 2:
                                self.parsed_results.append(parts[1])
                            else:
                                self.parsed_results.append("")
                        else:
                            self.parsed_results.append(line)
                
                print(f"成功加载 {len(self.parsed_results)} 个解析结果")
                return True
                
            except Exception as e:
                print(f"加载已有解析结果失败: {e}")
                print("重新执行解析步骤...")
                # 删除损坏的文件，重新解析
                os.remove(tmp_output_file)
        
        try:
            # 使用原有的prefix格式，但适配我们的数据结构
            prefix = """Please assist me in extracting valid information from a given natural language text and reconstructing it in JSON format, as demonstrated in the following example. If transportation details indicate a journey from one city to another (e.g., from A to B), the 'current_city' should be updated to the destination city (in this case, B). Use a ';' to separate different attractions, with each attraction formatted as 'Name, City'. If there's information about transportation, ensure that the 'current_city' aligns with the destination mentioned in the transportation details (i.e., the current city should follow the format 'from A to B'). Also, ensure that all flight numbers and costs are followed by a colon (i.e., 'Flight Number:' and 'Cost:'), consistent with the provided example. Each item should include ['day', 'current_city', 'transportation', 'breakfast', 'attraction', 'lunch', 'dinner', 'accommodation']. Replace non-specific information like 'eat at home/on the road' with '-'. Additionally, delete any '$' symbols.
-----EXAMPLE-----
 [{{
        "day": 1,
        "current_city": "from Dallas to Peoria",
        "transportation": "Flight Number: 4044830, from Dallas to Peoria, Departure Time: 13:10, Arrival Time: 15:01",
        "breakfast": "-",
        "attraction": "Peoria Historical Society, Peoria;Peoria Holocaust Memorial, Peoria;",
        "lunch": "-",
        "dinner": "Tandoor Ka Zaika, Peoria",
        "accommodation": "Bushwick Music Mansion, Peoria"
    }},
    {{
        "day": 2,
        "current_city": "Peoria",
        "transportation": "-",
        "breakfast": "Tandoor Ka Zaika, Peoria",
        "attraction": "Peoria Riverfront Park, Peoria;The Peoria PlayHouse, Peoria;Glen Oak Park, Peoria;",
        "lunch": "Cafe Hashtag LoL, Peoria",
        "dinner": "The Curzon Room - Maidens Hotel, Peoria",
        "accommodation": "Bushwick Music Mansion, Peoria"
    }},
    {{
        "day": 3,
        "current_city": "from Peoria to Dallas",
        "transportation": "Flight Number: 4045904, from Peoria to Dallas, Departure Time: 07:09, Arrival Time: 09:20",
        "breakfast": "-",
        "attraction": "-",
        "lunch": "-",
        "dinner": "-",
        "accommodation": "-"
    }}]
-----EXAMPLE END-----
"""
            
            # 直接从加载的模型输出数据构建提示，使用原有的格式
            prompt_list = []
            for item in self.model_outputs:
                model_output = item.get("model_output", "")
                if model_output and model_output not in ["", "Max Token Length Exceeded."]:
                    prompt = prefix + "Text:\n"+model_output+"\nJSON:\n"
                    prompt_list.append(prompt)
                else:
                    prompt_list.append("") # 保留空字符串以维持索引对应

            # 清除旧的解析结果文件
            if os.path.exists(tmp_output_file):
                os.remove(tmp_output_file)

            total_price = 0
            self.parsed_results = []
            
            print(f"处理 {len(prompt_list)} 个计划...")
            for idx, prompt in enumerate(tqdm(prompt_list)):
                if not prompt:
                    result = "" # 对于没有内容的计划，解析结果也为空
                    self.parsed_results.append(result)
                    # 不再写入空结果到文件，避免混淆
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
            print(f"处理 {len(self.model_outputs)} 个提取任务...")
            success_count = 0
            
            for idx, item in enumerate(tqdm(self.model_outputs)):
                # 检查是否有有效的模型输出和解析结果
                model_output = item.get("model_output", "")
                if model_output and model_output not in ["", "Max Token Length Exceeded."] and idx < len(self.parsed_results):
                    try:
                        # 提取 JSON 内容
                        result_text = self.parsed_results[idx]
                        if '```json' in result_text:
                            result = result_text.split('```json')[1].split('```')[0]
                        else:
                            result = result_text
                        
                        # 解析 JSON - 期望直接得到计划数组，而不是包含"plan"键的对象
                        try:
                            parsed_result = json.loads(result.strip())
                        except json.JSONDecodeError:
                            # 如果 json.loads 失败，尝试使用 eval 作为备选
                            parsed_result = eval(result.strip())
                        
                        # 验证解析结果的格式 - 应该直接是一个列表
                        if isinstance(parsed_result, list) and len(parsed_result) > 0:
                            # 检查列表中的第一个元素是否包含必要字段
                            if isinstance(parsed_result[0], dict) and 'current_city' in parsed_result[0]:
                                item['parsed_plan'] = parsed_result
                                success_count += 1
                            else:
                                print(f"第 {idx} 个计划格式错误: 列表元素缺少必要字段")
                                item['parsed_plan'] = None
                        elif isinstance(parsed_result, dict) and 'plan' in parsed_result:
                            # 如果意外得到了包含"plan"键的对象，提取其中的计划
                            plan_data = parsed_result['plan']
                            if isinstance(plan_data, list) and len(plan_data) > 0:
                                item['parsed_plan'] = plan_data
                                success_count += 1
                            else:
                                print(f"第 {idx} 个计划格式错误: plan 应该是非空列表")
                                item['parsed_plan'] = None
                        else:
                            print(f"第 {idx} 个计划格式错误: 应该是列表或包含'plan'键的对象")
                            item['parsed_plan'] = None
                        
                    except Exception as e:
                        print(f"解析第 {idx} 个结果时出错: {e}")
                        item['parsed_plan'] = None # 标记解析失败
                else:
                    item['parsed_plan'] = None # 没有有效输出或解析结果
            
            print(f"提取完成，成功处理: {success_count}/{len(self.model_outputs)}")
            return True
            
        except Exception as e:
            print(f"提取步骤失败: {e}")
            traceback.print_exc()
            return False
    
    def step3_combination(self):
        """步骤3: 合并为评估格式"""
        print("\n📊 步骤 3/4: 合并文件...")
        
        # 检查提交文件是否已经存在且非空
        if os.path.exists(self.submission_file) and os.path.getsize(self.submission_file) > 0:
            print(f"✅ 提交文件已存在且非空: {self.submission_file}")
            print("跳过步骤3，直接加载已有的提交文件...")
            
            # 加载已有的提交数据
            try:
                self.submission_list = self.load_line_json_data(self.submission_file)
                print(f"成功加载 {len(self.submission_list)} 个提交项")
                return True
                
            except Exception as e:
                print(f"加载已有提交文件失败: {e}")
                print("重新执行合并步骤...")
                # 删除损坏的文件，重新合并
                os.remove(self.submission_file)
        
        try:
            self.submission_list = []
            
            # 确保两个数据集的长度一致
            min_length = min(len(self.query_data_list), len(self.model_outputs))
            print(f"合并 {min_length} 个计划...")
            
            for idx in tqdm(range(min_length)):
                query_item = self.query_data_list[idx]
                model_item = self.model_outputs[idx]
                
                # 添加到提交列表，使用真正的查询数据和解析后的计划
                submission_item = {
                    "idx": query_item.get('idx', idx + 1), # 使用真正查询数据的idx
                    "query": query_item['query'],  # 使用真正的查询数据
                    "plan": model_item.get('parsed_plan', None) # 使用模型输出中解析好的计划
                }
                self.submission_list.append(submission_item)
            
            # 写入提交文件
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
            scores, detailed_scores, per_plan_results = self.eval_score_with_per_plan(self.set_type, self.submission_file)

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

            # 保存结果到文件
            self.save_results(scores, detailed_scores)

            # 新增：保存每条规划的详细评价指标到 jsonl 文件
            self.save_per_plan_results(per_plan_results)

            return True, scores, detailed_scores
        except Exception as e:
            print(f"评估步骤失败: {e}")
            traceback.print_exc()
            return False, None, None
        
    def save_per_plan_results(self, per_plan_results):
        """保存每条规划的详细评价指标到 jsonl 文件"""
        print("\n💾 保存每条规划的详细评价指标...")
        base_name = os.path.basename(self.args.input_file)
        if base_name.endswith('.json'):
            base_name = base_name[:-5]
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        per_plan_filename = f"{base_name}_per_results_{timestamp}.jsonl"
        per_plan_path = os.path.join(self.results_dir, per_plan_filename)
        with open(per_plan_path, 'w', encoding='utf-8') as f:
            for item in per_plan_results:
                f.write(json.dumps(item, ensure_ascii=False) + "\n")
        print(f"📁 每条规划详细评价已保存到: {per_plan_path}")
        return per_plan_path
    
    def eval_score_with_per_plan(self, set_type: str, file_path: str):
        """评估分数并返回每条规划的详细评价指标"""
        # 加载测试计划
        tested_plans = self.load_line_json_data(file_path)

        # 初始化统计变量
        hardConstraint_statistic = {level: {day: [] for day in [3,5,7]} for level in ['easy','medium','hard']}
        commonsenseConstraint_statistic = {level: {day: [] for day in [3,5,7]} for level in ['easy','medium','hard']}

        delivery_cnt = 0
        plan_constraint_store = []
        per_plan_results = []

        print(f"评估 {min(len(self.query_data_list), len(tested_plans))} 个计划...")

        for idx in range(0, min(len(self.query_data_list), len(tested_plans))):
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

            # 新增：每条规划的详细评价指标
            per_plan_results.append({
                'idx': query_data.get('idx', idx),
                'query': query_data.get('query', None),
                'plan': tested_plan.get('plan', None),
                'commonsense_constraint': commonsense_info_box,
                'hard_constraint': hard_info_box
            })

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
                                    if level in ['medium', 'hard']:
                                        constraint_dis_record[constraint]['total'] += mapping_constraint_record[level][day].get(key, 0)
                                    else:
                                        constraint_dis_record[constraint]['total'] += count_record[level][day]
                                else:
                                    constraint_dis_record[constraint]['total'] += count_record[level][day]

        # 计算宏观通过率
        for idx in range(0, min(len(self.query_data_list), len(plan_constraint_store))):
            if plan_constraint_store[idx]['commonsense_constraint']:
                final_commonsense_pass = True
                final_hardConstraint_pass = True
                for item in plan_constraint_store[idx]['commonsense_constraint']:
                    if plan_constraint_store[idx]['commonsense_constraint'][item][0] is not None and not plan_constraint_store[idx]['commonsense_constraint'][item][0]:
                        final_commonsense_pass = False
                        break
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
            total_count = min(len(self.query_data_list), len(tested_plans))
            commonsense_total = constraint_dis_record['commonsense']['total'] or total_count * 8
            hard_total = constraint_dis_record['hard']['total'] or 1

        result['Delivery Rate'] = delivery_cnt / total_count if total_count > 0 else 0
        result['Commonsense Constraint Micro Pass Rate'] = constraint_dis_record['commonsense']['pass'] / commonsense_total if commonsense_total > 0 else 0
        result['Commonsense Constraint Macro Pass Rate'] = final_commonsense_cnt / total_count if total_count > 0 else 0
        result['Hard Constraint Micro Pass Rate'] = constraint_dis_record['hard']['pass'] / hard_total if hard_total > 0 else 0
        result['Hard Constraint Macro Pass Rate'] = final_hardConstraint_cnt / total_count if total_count > 0 else 0
        result['Final Pass Rate'] = final_all_cnt / total_count if total_count > 0 else 0

        remap_commonsense_constraint_record, remap_hard_constraint_record = self.paper_term_mapping(
            commonsenseConstraint_statistic_processed, hardConstraint_statistic_processed)

        detailed_scores = {
            "Commonsense Constraint": remap_commonsense_constraint_record,
            "Hard Constraint": remap_hard_constraint_record
        }

        return result, detailed_scores, per_plan_results
    
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
        
        # 从输入文件名派生结果文件名
        base_name = os.path.basename(self.args.input_file)
        if base_name.endswith('.json'):
            base_name = base_name[:-5]
        
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
            'source_file': self.args.input_file,
            'timestamp': timestamp
        }
        
        combined_filename = f"{base_name}_results.json"
        combined_path = os.path.join(self.results_dir, combined_filename)
        
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
            # 步骤1: 解析
            if not self.step1_parsing():
                print("❌ 步骤1失败，停止执行")
                return False
            
            # 步骤2: 提取
            if not self.step2_element_extraction():
                print("❌ 步骤2失败，停止执行")
                return False
            
            # 步骤3: 合并
            if not self.step3_combination():
                print("❌ 步骤3失败，停止执行")
                return False
            
            # 步骤4: 评估
            success, _, _ = self.step4_evaluation()
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
    parser.add_argument("input_file", type=str,
                       help="包含模型生成内容的输入JSON文件路径 (例如: output/train_v0_20250819203412.json)")
    
    args = parser.parse_args()
    
    print("📋 评估配置:")
    print(f"  输入文件: {args.input_file}")
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
