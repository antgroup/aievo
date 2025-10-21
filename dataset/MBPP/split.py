#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
Split a JSONL dataset into a random train subset and the remaining eval subset.

Default behavior (when run from repo root or this folder):
- Input:   mbpp_validate.jsonl
- Output:  mbpp_train.jsonl (20 samples) and mbpp_eval.jsonl (rest)

Both output files receive an added integer field "id" starting from 0 and
increasing sequentially within each file.

Usage examples:
- python3 dataset/MBPP/split.py
- python3 dataset/MBPP/split.py --seed 42 --train-size 20
- python3 dataset/MBPP/split.py --input /abs/path/mbpp_validate.jsonl \
	--out-train /abs/path/mbpp_train.jsonl --out-eval /abs/path/mbpp_eval.jsonl
"""

from __future__ import annotations

import argparse
import json
import os
import random
import sys
from typing import List, Dict, Any


def _script_dir() -> str:
	return os.path.dirname(os.path.abspath(__file__))


def read_jsonl(path: str) -> List[Dict[str, Any]]:
	items: List[Dict[str, Any]] = []
	with open(path, "r", encoding="utf-8") as f:
		for line in f:
			line = line.strip()
			if not line:
				continue
			items.append(json.loads(line))
	return items


def _reorder_for_output(obj: Dict[str, Any]) -> Dict[str, Any]:
	"""Return a new dict with keys in desired order:
	1) id first
	2) all other keys except 'source_file' in their original relative order
	3) 'source_file' last if present
	"""
	# Capture original order
	keys = list(obj.keys())
	has_id = "id" in obj
	has_source = "source_file" in obj

	ordered: Dict[str, Any] = {}
	# id first
	if has_id:
		ordered["id"] = obj["id"]

	# other fields excluding id and source_file (preserve input order)
	for k in keys:
		if k == "id" or k == "source_file":
			continue
		ordered[k] = obj[k]

	# source_file last
	if has_source:
		ordered["source_file"] = obj["source_file"]

	return ordered


def write_jsonl(path: str, items: List[Dict[str, Any]]) -> None:
	os.makedirs(os.path.dirname(path), exist_ok=True)
	with open(path, "w", encoding="utf-8") as f:
		for obj in items:
			out_obj = _reorder_for_output(obj)
			f.write(json.dumps(out_obj, ensure_ascii=False) + "\n")


def add_sequential_ids(items: List[Dict[str, Any]], id_key: str = "id") -> None:
	for idx, obj in enumerate(items):
		obj[id_key] = idx


def parse_args(argv: List[str]) -> argparse.Namespace:
	d = _script_dir()
	default_input = os.path.join(d, "mbpp_validate.jsonl")
	default_out_train = os.path.join(d, "mbpp_train.jsonl")
	default_out_eval = os.path.join(d, "mbpp_eval.jsonl")

	parser = argparse.ArgumentParser(description="Randomly split JSONL into train/eval with sequential ids")
	parser.add_argument("--input", type=str, default=default_input, help="Path to input JSONL (default: mbpp_validate.jsonl next to this script)")
	parser.add_argument("--train-size", type=int, default=20, help="Number of samples in train subset (default: 20)")
	parser.add_argument("--out-train", type=str, default=default_out_train, help="Output path for train JSONL (default: mbpp_train.jsonl next to this script)")
	parser.add_argument("--out-eval", type=str, default=default_out_eval, help="Output path for eval JSONL (default: mbpp_eval.jsonl next to this script)")
	parser.add_argument("--seed", type=int, default=None, help="Random seed for reproducibility (default: None)")
	return parser.parse_args(argv)


def main(argv: List[str]) -> int:
	args = parse_args(argv)
	if args.seed is not None:
		random.seed(args.seed)

	if not os.path.isfile(args.input):
		print(f"[ERROR] Input file not found: {args.input}", file=sys.stderr)
		return 2

	data = read_jsonl(args.input)
	n = len(data)
	if n == 0:
		print("[WARN] Input has 0 items; producing empty outputs.")
	k = min(max(args.train_size, 0), n)

	# Sample k unique indices and keep original order for stability
	selected_indices = sorted(random.sample(range(n), k)) if k > 0 else []
	selected_set = set(selected_indices)

	train_items = [data[i] for i in selected_indices]
	eval_items = [obj for i, obj in enumerate(data) if i not in selected_set]

	# Add sequential ids starting from 0 within each file
	add_sequential_ids(train_items, id_key="id")
	add_sequential_ids(eval_items, id_key="id")

	write_jsonl(args.out_train, train_items)
	write_jsonl(args.out_eval, eval_items)

	print(
		f"Done. Total={n}, Train={len(train_items)}, Eval={len(eval_items)}\n"
		f"Train -> {args.out_train}\nEval  -> {args.out_eval}"
	)
	return 0


if __name__ == "__main__":
	raise SystemExit(main(sys.argv[1:]))

