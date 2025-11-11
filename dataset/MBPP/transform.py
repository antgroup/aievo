import argparse
import json
import os
import re
from typing import Any, Dict, List


def read_json(path: str):
	with open(path, 'r', encoding='utf-8') as f:
		return json.load(f)


def write_jsonl(path: str, rows: List[Dict[str, Any]]):
	os.makedirs(os.path.dirname(path), exist_ok=True)
	with open(path, 'w', encoding='utf-8') as f:
		for obj in rows:
			f.write(json.dumps(obj, ensure_ascii=False) + "\n")


def extract_entry_point_from_text(text: str) -> str:
	"""Extract the first function name from a python 'def' line in given text."""
	if not isinstance(text, str):
		return ""
	# simple regex for def functionName(
	m = re.search(r"\bdef\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(", text)
	return m.group(1) if m else ""


def extract_entry_point(code: str, prompt: str) -> str:
	# Prefer from code first, then fallback to prompt
	ep = extract_entry_point_from_text(code or "")
	if ep:
		return ep
	return extract_entry_point_from_text(prompt or "")


def transform_record(item: Dict[str, Any], idx: int) -> Dict[str, Any]:
	"""Transform one MBPP PRO item to standard MBPP jsonl schema."""
	task_id = item.get('id', idx)
	prompt = item.get('new_problem', '')
	code = item.get('new_solution', '')
	test_code = item.get('test_code', '') or ''
	# Indent test_code to be wrapped in a function
	indented_test_code = "    " + test_code.replace("\n", "\n    ")
	test_code = "def check():\n" + indented_test_code

	entry_point = extract_entry_point(code, prompt)
	# Split test_code by lines to form test_list (ignore empty lines)
	test_list = [ln for ln in (test_code.split('\n') if isinstance(test_code, str) else []) if ln.strip() and 'check()' not in ln]

	# Build standard row
	row = {
		"source_file": "MBPP_PRO",
		"task_id": task_id,
		"prompt": prompt,
		"code": code,
		"test_imports": [],  # not provided in pro set; keep empty
		"test_list": test_list,
		"entry_point": entry_point,
		"test": test_code,
	}
	return row


def main():
	parser = argparse.ArgumentParser(description="Transform mbpp_pro.json to mbpp_test.jsonl-compatible format")
	parser.add_argument('--input', '-i', type=str, default=os.path.join(os.path.dirname(__file__), 'mbpp_pro.json'),
						help='Path to mbpp_pro.json (array JSON)')
	parser.add_argument('--output', '-o', type=str, default=os.path.join(os.path.dirname(__file__), 'mbpp_pro.jsonl'),
						help='Path to output jsonl file')
	args = parser.parse_args()

	data = read_json(args.input)
	if not isinstance(data, list):
		raise ValueError("Input JSON must be an array of objects.")

	out_rows: List[Dict[str, Any]] = []
	for idx, item in enumerate(data):
		if not isinstance(item, dict):
			continue
		out_rows.append(transform_record(item, idx))

	write_jsonl(args.output, out_rows)
	print(f"Transformed {len(out_rows)} records -> {args.output}")


if __name__ == '__main__':
	main()

