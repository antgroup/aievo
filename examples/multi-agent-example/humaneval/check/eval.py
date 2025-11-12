import argparse
import json
import os
import sys
import traceback
from contextlib import redirect_stdout
import io
from multiprocessing import Process, Queue
from tqdm import tqdm

def read_jsonl(path):
    """Reads a .jsonl file and returns a list of dictionaries."""
    with open(path, 'r', encoding='utf-8') as f:
        return [json.loads(line) for line in f]

def read_json(path):
    """Reads a .json file and returns a dictionary or list."""
    with open(path, 'r', encoding='utf-8') as f:
        return json.load(f)

def _strip_code_fences(code: str) -> str:
    """Remove Markdown code fences like ```python ... ``` or ``` ... ``` around the model output."""
    if not isinstance(code, str):
        return code
    text = code.strip()
    if text.startswith("```"):
        lines = text.splitlines()
        # drop first fence line
        if lines and lines[0].lstrip().startswith("```"):
            lines = lines[1:]
        # drop last fence line if present
        if lines and lines[-1].strip() == "```":
            lines = lines[:-1]
        text = "\n".join(lines).strip()
    return text

def _build_full_code(prompt: str, generated_code: str, test: str, entry_point: str, mode: int) -> str:
    """Construct executable code so that a single function named `candidate` is defined,
    while keeping helper functions from prompt and providing a back-compat alias for tests.

    Changes vs. previous behavior:
    - Always include the prompt so helper functions (e.g., encode_cyclic) are available.
    - Rename `def {entry_point}(` from either prompt or generated code to `def candidate(`.
    - Add `{entry_point} = candidate` alias so tests that reference the original name don't fail.
    """
    # Prepare a candidate-style prompt header (no canonical solution in prompt)
    # Only rename the entrypoint signature; other helpers remain.
    import re

    def sanitize_prompt_to_code(p: str) -> str:
        # 去掉代码围栏，并把自然语言段落注释化，直到出现代码行
        p = _strip_code_fences(p or "")
        lines = p.splitlines()
        out = []
        code_started = False
        for ln in lines:
            s = ln.lstrip()
            if not code_started:
                if not s:
                    out.append("")  # 保留空行
                    continue
                if s.startswith(("#", "def ", "from ", "import ", "@")):
                    code_started = True
                    out.append(ln)
                    continue
                # 普通自然语言行，注释化
                out.append("# " + ln if not ln.startswith("#") else ln)
                continue
            out.append(ln)
        return "\n".join(out).strip()

    if mode == 1:
        # prompt = sanitize_prompt_to_code(prompt)
        # print("Sanitized prompt for PRO mode.")
        # print(prompt)
        # print("----- End of sanitized prompt -----")
        prompt = ""

    prompt_candidate = prompt.replace(f"def {entry_point}", "def candidate")

    def indent_body(body: str) -> str:
        lines = body.splitlines()
        indented = []
        for ln in lines:
            if ln.strip() == "":
                indented.append(ln)
            elif ln.startswith("\t") or ln.startswith("    "):
                indented.append(ln)
            else:
                indented.append("    " + ln)
        return "\n".join(indented)

    # Clean possible Markdown code fences from model output first
    generated_code = _strip_code_fences(generated_code)

    alias_line = f"\n{entry_point} = candidate\n"

    # Heuristic: if generated code defines a function, rely on it and just rename.
    if "def " in generated_code:
        gen = generated_code
        # Replace the first occurrence of the entry point def to candidate.
        gen = gen.replace(f"def {entry_point}(", "def candidate(")
        # Also handle potential spaces before parenthesis (rare, but safe)
        gen = gen.replace(f"def {entry_point} (", "def candidate (")
        # Always include prompt first (to provide helpers), then model code, alias, then tests
        full = prompt_candidate + "\n" + gen + alias_line + test
    else:
        # Treat as body and attach under the prompt header (now named candidate).
        body = indent_body(generated_code)
        full = prompt_candidate + "\n" + body + alias_line + test
    # print(full)
    # exit(0)
    return full


def _worker_run(full_code_str: str, q: Queue):
    """Worker process: exec combined code, then run check(candidate). Put result into Queue."""
    try:
        # Suppress prints from test/code to avoid polluting parent output
        buf = io.StringIO()
        with redirect_stdout(buf):
            exec_globals = {}
            # Define symbols
            exec(full_code_str, exec_globals)
            # Check required symbols
            candidate_fn = exec_globals.get("candidate")
            check_fn = exec_globals.get("check")
            if candidate_fn is None or check_fn is None:
                missing = []
                if candidate_fn is None:
                    missing.append("candidate")
                if check_fn is None:
                    missing.append("check")
                q.put({
                    "status": "fail",
                    "error": f"Missing required symbol(s): {', '.join(missing)}"
                })
                return

            try:
                check_fn(candidate_fn)
                q.put({"status": "pass"})
            except Exception:
                error_info = traceback.format_exc()
                q.put({
                    "status": "fail",
                    "error": f"Test failed:\n{error_info}"
                })
    except Exception:
        error_info = traceback.format_exc()
        q.put({
            "status": "fail",
            "error": f"Execution failed:\n{error_info}"
        })


def check_correctness(prompt: str, generated_code: str, test: str, entry_point: str, timeout: int = 30, mode: int = 0) -> dict:
    """
    Evaluates the generated code against the test cases.

    Args:
        prompt (str): The function signature and docstring, with the function name replaced by 'candidate'.
        generated_code (str): The code generated by the model (the function body).
        test (str): The test code from the dataset.

    Returns:
        dict: A dictionary containing the status ('pass' or 'fail') and any error information.
    """
    # Combine the function signature, the generated body, and the test code into a single script.
    # The prompt provides the 'def candidate(...):' line.
    # The generated_code is the indented body.
    # The test code contains the checks.
    full_code_str = _build_full_code(prompt, generated_code, test, entry_point, mode)

    # Run evaluation in a separate process and enforce timeout
    q: Queue = Queue()
    p = Process(target=_worker_run, args=(full_code_str, q))
    p.start()
    p.join(timeout)
    if p.is_alive():
        # Timeout
        p.terminate()
        p.join()
        return {
            "status": "fail",
            "error": f"Timeout: evaluation exceeded {timeout}s"
        }
    # Process finished; attempt to retrieve result
    if not q.empty():
        return q.get()
    # Shouldn't happen, but guard just in case
    return {
        "status": "fail",
        "error": "Unknown error: no result returned from worker"
    }


def main():
    """
    Main function to run the evaluation.
    """
    parser = argparse.ArgumentParser(description="Evaluate HumanEval model outputs.")
    parser.add_argument(
        '--model_output_path',
        type=str,
        required=True,
        help='Path to the model output JSON file.'
    )
    parser.add_argument(
        '--timeout',
        type=int,
        default=30,
        help='Per-problem evaluation timeout in seconds (default: 30).'
    )
    args = parser.parse_args()

    # --- Configuration ---
    model_output_path = args.model_output_path
    per_problem_timeout = args.timeout
    mode = 0
    if 'train' in model_output_path:
        dataset_path = '../../../../dataset/humaneval/train.jsonl'
    elif 'valid' in model_output_path or 'eval' in model_output_path:
        dataset_path = '../../../../dataset/humaneval/valid.jsonl'
    elif 'pro' in model_output_path:
        dataset_path = '../../../../dataset/humaneval/pro.jsonl'
        mode = 1
    else:
        dataset_path = '../../../../dataset/humaneval/test.jsonl'
    results_output_path = '../results/' + model_output_path[10:] + '_results.jsonl'
    # --- End Configuration ---

    if not os.path.exists(model_output_path):
        print(f"Error: Model output file not found at '{model_output_path}'")
        sys.exit(1)

    model_outputs = read_json(model_output_path)
    dataset = read_jsonl(dataset_path)
    n_true = 0
    n_total = 0

    if len(model_outputs) > len(dataset):
        print(f"Warning: There are more model outputs ({len(model_outputs)}) than dataset problems ({len(dataset)}).")
        print("Evaluation will only run for the available problems in the dataset.")

    with open(results_output_path, 'w', encoding='utf-8') as f_out:
        for i, model_result in tqdm(enumerate(model_outputs), total=len(model_outputs)):
            if i >= len(dataset):
                break
            
            # print(f"\n=============Evaluating Problem {i}...")
            n_total += 1
            problem = dataset[i]
            
            task_id = problem.get("task_id")
            prompt = model_result.get("query") # The prompt was saved in the 'query' field
            generated_code = model_result.get("model_output")
            test_code = problem.get("test")
            entry_point = problem.get("entry_point")

            if not all([task_id, prompt, generated_code, test_code, entry_point]):
                print(f"Skipping problem {i} due to missing data.")
                continue
            
            # 标准化测试用例中的入口为 candidate，函数定义按构建器处理
            prompt_for_eval = prompt
            test_for_eval = test_code.replace(f"check({entry_point})", "check(candidate)")

            # print(f"Evaluating Task ID: {task_id}...")
            
            result = check_correctness(prompt_for_eval, generated_code, test_for_eval, entry_point, timeout=per_problem_timeout, mode=mode)
            
            # print(f"Result: {result['status'].upper()}")
            n_true += (result['status'] == 'pass')

            # Write the result to the output file
            eval_log = {
                "id": i,
                "task_id": task_id,
                "status": result['status'],
                "error": result.get("error", None)
            }
            f_out.write(json.dumps(eval_log) + '\n')

        f_out.write(f'\nTotal Problems Evaluated: {n_total}, {n_true} Passed. Success Rate: {n_true / n_total:.3%}\n')
    
    print(f"\nTotal Problems Evaluated: {n_total}, {n_true} Passed. Success Rate: {n_true / n_total:.3%}")
    print(f"Evaluation complete. Results saved to '{results_output_path}'")

if __name__ == '__main__':
    main()
