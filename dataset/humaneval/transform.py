import argparse
import json
import os
import re
from typing import Any, Dict, List, Optional, Tuple


def read_json(path: str):
    with open(path, 'r', encoding='utf-8') as f:
        return json.load(f)


def write_jsonl(path: str, rows: List[Dict[str, Any]]):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w', encoding='utf-8') as f:
        for obj in rows:
            f.write(json.dumps(obj, ensure_ascii=False) + "\n")


# ------------ helpers: parsing ---------------

_DEF_NAME_RE = re.compile(r"\bdef\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(")
_ASSERT_CALL_RE = re.compile(r"^\s*assert\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(", re.M)


def extract_first_def_name(text: str) -> str:
    if not isinstance(text, str):
        return ""
    m = _DEF_NAME_RE.search(text)
    return m.group(1) if m else ""


def extract_entry_point_from_tests(test_code: str) -> Optional[str]:
    """
    Pick the most frequent function name called at the start of assert lines.
    """
    if not isinstance(test_code, str):
        return None
    names = _ASSERT_CALL_RE.findall(test_code)
    if not names:
        return None
    # choose most common
    freq: Dict[str, int] = {}
    for n in names:
        freq[n] = freq.get(n, 0) + 1
    return sorted(freq.items(), key=lambda kv: (-kv[1], kv[0]))[0][0]


def find_signature_for_ep(ep: str, new_problem: str, new_solution: str) -> Optional[str]:
    """
    Return the 'def ep(...) :' line (without following body).
    Try new_problem first; then new_solution.
    """
    pattern = re.compile(rf"^\s*def\s+{re.escape(ep)}\s*\(.*\):\s*$", re.M)
    for src in (new_problem or "", new_solution or ""):
        m = pattern.search(src)
        if m:
            return m.group(0)
    return None


def extract_body_for_ep(ep: str, new_solution: str) -> str:
    """
    Extract the function body (indented block) for def ep(...) from new_solution.
    If exact def not found, fallback to initial indented block.
    """
    if not isinstance(new_solution, str):
        return ""

    lines = new_solution.splitlines()
    # Try to locate 'def ep(' line
    def_line_idx = None
    def_indent = 0
    def_line_pattern = re.compile(rf"^\s*def\s+{re.escape(ep)}\s*\(.*\):\s*$")
    for i, ln in enumerate(lines):
        if def_line_pattern.match(ln):
            def_line_idx = i
            def_indent = len(ln) - len(ln.lstrip(" "))
            break

    body_lines: List[str] = []

    if def_line_idx is not None:
        # Collect the indented block after def line
        i = def_line_idx + 1
        while i < len(lines):
            ln = lines[i]
            # blank lines are part of body
            if ln.strip() == "":
                body_lines.append(ln)
                i += 1
                continue
            indent = len(ln) - len(ln.lstrip(" "))
            if indent <= def_indent:
                break
            body_lines.append(ln)
            i += 1
    else:
        # Fallback: take leading indented block (common in provided new_solution)
        for ln in lines:
            if ln.strip() == "":
                # keep leading blanks within body until we hit non-indented or unindented line
                body_lines.append(ln)
                continue
            if ln.startswith("    "):  # 4-space indented
                body_lines.append(ln)
            else:
                # stop when first non-indented line encountered
                if body_lines:
                    break

    # Ensure body is not empty
    body = "\n".join(body_lines).rstrip("\n")
    return body


def build_test_block(ep: str, test_code: str) -> str:
    """
    Wrap test_code inside HumanEval-style check(candidate).
    Replace top-level target function calls in asserts with candidate(...).
    Keep any imports or helper stmts inside check.
    """
    if not isinstance(test_code, str):
        test_code = ""

    # Collect function names to replace from assert heads
    names = list(set(_ASSERT_CALL_RE.findall(test_code)))
    # Replace each occurrence in assert heads only
    def repl_assert_heads(match: re.Match) -> str:
        # match.groups()[0] is the function name; replace with 'candidate'
        return match.group(0).replace(match.group(1), "candidate", 1)

    replaced = _ASSERT_CALL_RE.sub(repl_assert_heads, test_code)

    # Indent inside check
    indented = "\n".join(("    " + ln if ln.strip() != "" else "    ") for ln in replaced.strip("\n").splitlines())

    test_block = (
        "\n\nMETADATA = {}\n\n\n"
        "def check(candidate):\n"
        f"{indented}\n"
    )
    return test_block


# ------------- main transform logic ----------------

def transform_record(item: Dict[str, Any], idx: int) -> Dict[str, Any]:
    """
    Transform one HumanEval PRO item to HumanEval train.jsonl schema.
    Fields:
    - task_id: "HumanEval/{id}"
    - prompt: raw_problem + two newlines + new_problem (+ def ep(...) stub if missing)
    - canonical_solution: body of entry point function (no 'def' line)
    - test: HumanEval-style 'METADATA' + 'def check(candidate):' with asserts calling candidate(...)
    - entry_point: inferred from test_code, fallback to first def in new_problem
    """
    item_id = item.get('id', idx)
    task_id = f"{item_id}"

    raw_problem = (item.get('raw_problem') or "").rstrip()
    new_problem = (item.get('new_problem') or "").rstrip()
    new_solution = item.get('new_solution') or ""
    test_code = item.get('test_code') or ""

    # Determine entry_point
    ep = extract_first_def_name(new_problem) or ""

    # Find signature for ep
    signature = find_signature_for_ep(ep, new_problem, new_solution)
    # Build prompt: raw_problem + blank line + new_problem (if present)
    prompt_parts: List[str] = []
    if raw_problem:
        prompt_parts.append(raw_problem.strip("\n"))
    # Append new_problem as-is
    if new_problem:
        prompt_parts.append(new_problem.strip("\n"))

    # If ep signature not present in prompt text, try to append it as a stub line
    prompt_text = "\n\n".join(prompt_parts).strip("\n")
    if ep and signature and (signature not in prompt_text):
        # Ensure separation
        if prompt_text:
            prompt_text += "\n"
        prompt_text += "\n" + signature + "\n"

    # canonical_solution: only the body of ep
    canonical_solution = extract_body_for_ep(ep, new_solution)

    # test: wrap into check(candidate)
    test_block = build_test_block(ep, test_code)

    # entry_point
    entry_point = ep

    row = {
        "task_id": task_id,
        "prompt": new_problem,
        "canonical_solution": canonical_solution,
        "test": test_block,
        "entry_point": entry_point,
    }
    return row


def main():
    parser = argparse.ArgumentParser(description="Transform humaneval_pro.json to HumanEval train.jsonl-compatible format")
    parser.add_argument(
        '--input',
        '-i',
        type=str,
        default=os.path.join(os.path.dirname(__file__), 'humaneval_pro.json'),
        help='Path to humaneval_pro.json (array JSON)'
    )
    parser.add_argument(
        '--output',
        '-o',
        type=str,
        default=os.path.join(os.path.dirname(__file__), 'humaneval_pro.jsonl'),
        help='Path to output jsonl file'
    )
    args = parser.parse_args()

    data = read_json(args.input)
    if not isinstance(data, list):
        raise ValueError("Input JSON must be an array of objects.")

    out_rows: List[Dict[str, Any]] = []
    for idx, item in enumerate(data):
        if not isinstance(item, dict):
            continue
        try:
            row = transform_record(item, idx)
            out_rows.append(row)
        except Exception as e:
            # Best-effort conversion; skip malformed entries but log minimal info
            print(f"[warn] skip index {idx} due to error: {e}")

    write_jsonl(args.output, out_rows)
    print(f"Transformed {len(out_rows)} records -> {args.output}")


if __name__ == '__main__':
    main()