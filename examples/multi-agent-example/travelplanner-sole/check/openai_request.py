import os
import openai
import math
import sys
import time
from tqdm import tqdm
from typing import Iterable, List, TypeVar
import func_timeout
from func_timeout import func_set_timeout
import json
from datasets import load_dataset

# 添加对我们的LLM调用函数的导入
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "../tools/planner")))
# from tools.planner.apis import safe_llm_call

# 修复LangChain导入警告 - 使用最新的langchain-openai包
try:
    from langchain_openai import ChatOpenAI
except ImportError:
    try:
        from langchain.chat_models import ChatOpenAI
    except ImportError:
        print("Warning: Could not import ChatOpenAI")

try:
    from langchain_core.messages import HumanMessage, SystemMessage
except ImportError:
    try:
        from langchain.schema import HumanMessage, SystemMessage
    except ImportError:
        print("Warning: Could not import message types")


T = TypeVar('T')
KEY_INDEX = 0
KEY_POOL =  [
   os.environ['OPENAI_API_KEY']
]# your key pool
openai.api_key = KEY_POOL[0]

# 新的LLM调用函数，使用环境变量和safe_llm_call
def create_llm_instance():
    """创建LLM实例，使用环境变量配置"""
    # model_name = os.environ.get('OPENAI_MODEL', 'gpt-4-1106-preview')
    model_name = "Qwen2.5-72B-Instruct"
    base_url = os.environ.get('OPENAI_BASE_URL', 'https://api.openai.com/v1')
    api_key = os.environ.get('OPENAI_API_KEY')
    
    try:
        llm = ChatOpenAI(
            model=model_name,
            openai_api_key=api_key,
            openai_api_base=base_url,
            temperature=0
        )
        return llm, model_name
    except Exception as e:
        print(f"Error creating LLM instance: {e}")
        return None, model_name

def safe_llm_call_local(llm, messages):
    """本地版本的safe_llm_call，用于parsing阶段"""
    try:
        if hasattr(llm, 'invoke'):
            response = llm.invoke(messages)
        else:
            response = llm(messages)
        return response
    except Exception as e:
        try:
            response = llm(messages)
            return response
        except Exception as e2:
            raise e


class TimeoutError(Exception):
    pass

def timeout_handler(signum, frame):
    raise TimeoutError("The function takes too long to run")

@func_set_timeout(120)
def limited_execution_time(func,model,prompt,temp,max_tokens=2048, default=None,**kwargs):
    try:
        if 'gpt-3.5-turbo' in model or 'gpt-4' in model:
            result = func(
                    model=model,
                    messages=prompt,
                    temperature=temp
                )
        else:
            result = func(model=model,prompt=prompt,max_tokens=max_tokens,**kwargs)
    except func_timeout.exceptions.FunctionTimedOut:
        return None
    # raise any other exception
    except Exception as e:
        raise e
    return result


def batchify(data: Iterable[T], batch_size: int) -> Iterable[List[T]]:
    # function copied from allenai/real-toxicity-prompts
    assert batch_size > 0
    batch = []
    for item in data:
        # Yield next batch
        if len(batch) == batch_size:
            yield batch
            batch = []
        batch.append(item)

    # Yield last un-filled batch
    if len(batch) != 0:
        yield batch


def openai_unit_price(model_name,token_type="prompt"):
    if 'gpt-4' in model_name:
        if token_type=="prompt":
            unit = 0.03
        elif token_type=="completion":
            unit = 0.06
        else:
            raise ValueError("Unknown type")
    elif 'gpt-3.5-turbo' in model_name:
        unit = 0.002
    elif 'davinci' in model_name:
        unit = 0.02
    elif 'curie' in model_name:
        unit = 0.002
    elif 'babbage' in model_name:
        unit = 0.0005
    elif 'ada' in model_name:
        unit = 0.0004
    else:
        unit = -1
    return unit


def calc_cost_w_tokens(total_tokens: int, model_name: str):
    unit = openai_unit_price(model_name,token_type="completion")
    return round(unit * total_tokens / 1000, 4)


def calc_cost_w_prompt(total_tokens: int, model_name: str):
    # 750 words == 1000 tokens
    unit = openai_unit_price(model_name)
    return round(unit * total_tokens / 1000, 4)


def get_perplexity(logprobs):
    assert len(logprobs) > 0, logprobs
    return math.exp(-sum(logprobs)/len(logprobs))


def keep_logprobs_before_eos(tokens, logprobs):
    keep_tokens = []
    keep_logprobs = []
    start_flag = False
    for tok, lp in zip(tokens, logprobs):
        if start_flag:
            if tok == "<|endoftext|>":
                break
            else:
                keep_tokens.append(tok)
                keep_logprobs.append(lp)
        else:
            if tok != '\n':
                start_flag = True
                if tok != "<|endoftext>":
                    keep_tokens.append(tok)
                    keep_logprobs.append(lp)

    return keep_tokens, keep_logprobs


def catch_openai_api_error(prompt_input: list):
    global KEY_INDEX
    error = sys.exc_info()[0]
    if error == openai.error.InvalidRequestError:
        # something is wrong: e.g. prompt too long
        print(f"InvalidRequestError\nPrompt:\n\n{prompt_input}\n\n")
        assert False
    elif error == openai.error.RateLimitError:
        KEY_INDEX = (KEY_INDEX + 1) % len(KEY_POOL)
        openai.api_key = KEY_POOL[KEY_INDEX]
        print("RateLimitError, now change the key. Current key is ", openai.api_key)
    elif error == openai.error.APIError:
        KEY_INDEX = (KEY_INDEX + 1) % len(KEY_POOL)
        openai.api_key = KEY_POOL[KEY_INDEX]
        print("APIError, now change the key. Current key is ", openai.api_key)
    elif error == openai.error.AuthenticationError:
        KEY_INDEX = (KEY_INDEX + 1) % len(KEY_POOL)
        openai.api_key = KEY_POOL[KEY_INDEX]
        print("AuthenticationError, now change the key. Current key is ", openai.api_key)
    elif error == TimeoutError:
        KEY_INDEX = (KEY_INDEX + 1) % len(KEY_POOL)
        openai.api_key = KEY_POOL[KEY_INDEX]
        print("TimeoutError, retrying...")
    else:
        print("API error:", error)


def prompt_gpt3(prompt_input: list, save_path,model_name='text-davinci-003', max_tokens=2048,
                clean=False, batch_size=16, verbose=False, **kwargs):
    # return: output_list, money_cost

    def request_api(prompts: list):
        # prompts: list or str

        total_tokens = 0
        results = []
        for batch in tqdm(batchify(prompt_input, batch_size), total=len(prompt_input) // batch_size):
            batch_response = request_api(batch)
            total_tokens += batch_response['usage']['total_tokens']
            if not clean:
                results += batch_response['choices']
            else:
                results += [choice['text'] for choice in batch_response['choices']]
            with open(save_path,'w+',encoding='utf-8') as f:
                for content in results:
                    content = content.replace("\n"," ")
                    f.write(content+'\n')
        return results, calc_cost_w_tokens(total_tokens, model_name)



def prompt_chatgpt(system_input, user_input, temperature, save_path, index, history=[], model_name=None):
    '''
    :param system_input: "You are a helpful assistant/translator."
    :param user_input: you texts here
    :param temperature: temperature for generation
    :param save_path: path to save results
    :param index: index for the request
    :param history: conversation history
    :param model_name: model name (will be overridden by environment variable)
    return: assistant_output, (updated) history, money cost
    '''
    # 使用环境变量中的模型配置
    llm, actual_model_name = create_llm_instance()
    if llm is None:
        # 回退到原始的OpenAI API调用
        return prompt_chatgpt_fallback(system_input, user_input, temperature, save_path, index, history, model_name)
    
    # 构建消息列表
    messages = [SystemMessage(content=system_input), HumanMessage(content=user_input)]
    
    try:
        # 使用safe_llm_call调用LLM
        response = safe_llm_call_local(llm, messages)
        assistant_output = response.content
        
        # 保存结果到文件
        with open(save_path, 'a+', encoding='utf-8') as f:
            formatted_output = str(index) + "\t" + "\t".join(x for x in assistant_output.split("\n"))
            f.write(formatted_output + '\n')
        
        # 更新历史记录
        if len(history) == 0:
            history = [{"role": "system", "content": system_input}]
        history.append({"role": "user", "content": user_input})
        history.append({"role": "assistant", "content": assistant_output})
        
        # 估算成本（简化版本）
        estimated_cost = 0.0  # 可以根据需要实现更精确的成本计算
        
        return assistant_output, history, estimated_cost
        
    except Exception as e:
        print(f"Error in LLM call: {e}")
        # 回退到原始方法
        return prompt_chatgpt_fallback(system_input, user_input, temperature, save_path, index, history, model_name)

def prompt_chatgpt_fallback(system_input, user_input, temperature, save_path, index, history=[], model_name='gpt-4-1106-preview'):
    '''
    原始的prompt_chatgpt函数作为回退方案
    '''
    if len(history) == 0:
        history = [{"role": "system", "content": system_input}]
    history.append({"role": "user", "content": user_input})
    while True:
        try:
            completion = limited_execution_time(openai.ChatCompletion.create,
                model=model_name,
                prompt=history,
                temp=temperature)
            if completion is None:
                raise TimeoutError
            break
        except:
            catch_openai_api_error(user_input)
            time.sleep(1)

    assistant_output = completion['choices'][0]['message']['content']
    history.append({"role": "assistant", "content": assistant_output})
    total_prompt_tokens = completion['usage']['prompt_tokens']
    total_completion_tokens = completion['usage']['completion_tokens']
    with open(save_path,'a+',encoding='utf-8') as f:
        assistant_output = str(index)+"\t"+"\t".join(x for x in assistant_output.split("\n"))
        f.write(assistant_output+'\n')
    return assistant_output, history, calc_cost_w_tokens(total_prompt_tokens, model_name) + calc_cost_w_prompt(total_completion_tokens, model_name)

def build_query_generation_prompt(data):
    prompt_list = []
    prefix = """Given a JSON, please help me generate a natural language query. In the JSON, 'org' denotes the departure city. When 'days' exceeds 3, 'visiting_city_number' specifies the number of cities to be covered in the destination state. Please disregard the 'level' attribute. Here are three examples.

-----EXAMPLE 1-----
JSON:
{"org": "Gulfport", "dest": "Charlotte", "days": 3, "visiting_city_number": 1, "date": ["2022-03-05", "2022-03-06", "2022-03-07"], "people_number": 1, "local_constraint": {"house rule": null, "cuisine": null, "room type": null}, "budget": 1800, "query": null, "level": "easy"}
QUERY:
Please design a travel plan departing Gulfport and heading to Charlotte for 3 days, spanning March 5th to March 7th, 2022, with a budget of $1800.
-----EXAMPLE 2-----
JSON:
{"org": "Omaha", "dest": "Colorado", "days": 5, "visiting_city_number": 2, "date": ["2022-03-14", "2022-03-15", "2022-03-16", "2022-03-17", "2022-03-18"], "people_number": 7, "local_constraint": {"house rule": "pets", "cuisine": null, "room type": null}, "budget": 35300, "query": null, "level": "medium"}
QUERY:
Could you provide a  5-day travel itinerary for a group of 7, starting in Omaha and exploring 2 cities in Colorado between March 14th and March 18th, 2022? Our budget is set at $35,300, and it's essential that our accommodations be pet-friendly since we're bringing our pets.
-----EXAMPLE 3-----
JSON:
{"org": "Indianapolis", "dest": "Georgia", "days": 7, "visiting_city_number": 3, "date": ["2022-03-01", "2022-03-02", "2022-03-03", "2022-03-04", "2022-03-05", "2022-03-06", "2022-03-07"], "people_number": 2, "local_constraint": {"flight time": null, "house rule": null, "cuisine": ["Bakery", "Indian"], "room type": "entire room", "transportation": "self driving"}, "budget": 6200, "query": null, "level": "hard"}
QUERY:
I'm looking for a week-long travel itinerary for 2 individuals. Our journey starts in Indianapolis, and we intend to explore 3 distinct cities in Georgia from March 1st to March 7th, 2022. Our budget is capped at $6,200. For our accommodations, we'd prefer an entire room. We plan to navigate our journey via self-driving. In terms of food, we're enthusiasts of bakery items, and we'd also appreciate indulging in genuine Indian cuisine.

JSON\n"""
    for unit in data:
        unit = str(unit).replace(", 'level': 'easy'",'').replace(", 'level': 'medium'",'').replace(", 'level': 'hard'",'')
        prompt = prefix + str(unit) + "\nQUERY\n"
        prompt_list.append(prompt)
    return prompt_list

def build_plan_format_conversion_prompt(directory, set_type='validation',model_name='gpt4',strategy='direct',mode='two-stage'):
    prompt_list = []
    prefix = """Please assist me in extracting valid information from a given natural language text and reconstructing it in JSON format, as demonstrated in the following example. If transportation details indicate a journey from one city to another (e.g., from A to B), the 'current_city' should be updated to the destination city (in this case, B). Use a ';' to separate different attractions, with each attraction formatted as 'Name, City'. If there's information about transportation, ensure that the 'current_city' aligns with the destination mentioned in the transportation details (i.e., the current city should follow the format 'from A to B'). Also, ensure that all flight numbers and costs are followed by a colon (i.e., 'Flight Number:' and 'Cost:'), consistent with the provided example. Each item should include ['day', 'current_city', 'transportation', 'breakfast', 'attraction', 'lunch', 'dinner', 'accommodation']. Replace non-specific information like 'eat at home/on the road' with '-'. Additionally, delete any '$' symbols.
-----EXAMPLE-----
 [{{
        "days": 1,
        "current_city": "from Dallas to Peoria",
        "transportation": "Flight Number: 4044830, from Dallas to Peoria, Departure Time: 13:10, Arrival Time: 15:01",
        "breakfast": "-",
        "attraction": "Peoria Historical Society, Peoria;Peoria Holocaust Memorial, Peoria;",
        "lunch": "-",
        "dinner": "Tandoor Ka Zaika, Peoria",
        "accommodation": "Bushwick Music Mansion, Peoria"
    }},
    {{
        "days": 2,
        "current_city": "Peoria",
        "transportation": "-",
        "breakfast": "Tandoor Ka Zaika, Peoria",
        "attraction": "Peoria Riverfront Park, Peoria;The Peoria PlayHouse, Peoria;Glen Oak Park, Peoria;",
        "lunch": "Cafe Hashtag LoL, Peoria",
        "dinner": "The Curzon Room - Maidens Hotel, Peoria",
        "accommodation": "Bushwick Music Mansion, Peoria"
    }},
    {{
        "days": 3,
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
    # if set_type == 'train':
    #     query_data_list  = load_dataset('osunlp/TravelPlanner','train')['train']
    # elif set_type == 'validation':
    #     query_data_list  = load_dataset('osunlp/TravelPlanner','validation')['validation']
    # elif set_type == 'test':
    #     query_data_list  = load_dataset('osunlp/TravelPlanner','test')['test']


    # directory = f'{args.output_dir}/{args.set_type}'
    # 设置数据集存储目录和文件路径
    dataset_dir = f'{directory}/datasets/{set_type}'
    dataset_file = os.path.join(dataset_dir, f'travelplanner_{set_type}_dataset.json')
    # metadata_file = os.path.join(dataset_dir, f'travelplanner_{args.set_type}_metadata.json')
    
    # 检查本地是否已有数据集文件
    if os.path.exists(dataset_file):
        print(f"Loading dataset from local file: {dataset_file}")
        try:
            with open(dataset_file, 'r', encoding='utf-8') as f:
                query_data_list = json.load(f)
            print(f"Successfully loaded {len(query_data_list)} items from local dataset")
            # logging.info(f"Dataset loaded from local file. Total items: {len(query_data_list)}")
        except Exception as e:
            print(f"Error loading local dataset: {e}")
            print("Falling back to downloading from HuggingFace...")
            query_data_list = None
    else:
        print(f"Local dataset not found at {dataset_file}")
        query_data_list = None
    
    # 如果本地没有数据集或加载失败，则从网络下载
    if query_data_list is None:
        print(f"Downloading dataset from HuggingFace: osunlp/TravelPlanner [{set_type}]")
        if set_type == 'validation':
            hf_dataset = load_dataset('osunlp/TravelPlanner', 'validation')['validation']
        elif set_type == 'test':
            hf_dataset = load_dataset('osunlp/TravelPlanner', 'test')['test']
        elif set_type == 'train':
            hf_dataset = load_dataset('osunlp/TravelPlanner', 'train')['train']
        else:
            raise ValueError(f"Unsupported set_type: {set_type}")
        
        # 转换为列表格式
        query_data_list = [dict(item) for item in hf_dataset]
        print(f"Downloaded {len(query_data_list)} items from HuggingFace")
        

    idx_number_list = [i for i in range(1,len(query_data_list)+1)]
    # idx_number_list = [i for i in range(1, 20)]
    if mode == 'two-stage':
        suffix = ''
    elif mode == 'sole-planning':
        suffix = f'_{strategy}'
    for idx in tqdm(idx_number_list):
        generated_plan = json.load(open(f'{directory}/{set_type}/generated_plan_{idx}.json'))
        if generated_plan[-1][f'{model_name}{suffix}_{mode}_results'] and generated_plan[-1][f'{model_name}{suffix}_{mode}_results'] != "":
            prompt = prefix + "Text:\n"+generated_plan[-1][f'{model_name}{suffix}_{mode}_results']+"\nJSON:\n"
        else:
            prompt = ""
        prompt_list.append(prompt)
    return prompt_list
