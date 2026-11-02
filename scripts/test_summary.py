#!/usr/bin/env python3
"""Summarize go test -json output and reject failures or skipped tests."""
import argparse
import json
from collections import Counter
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("log", type=Path)
args = parser.parse_args()
tests, top, packages = Counter(), Counter(), Counter()
for line in args.log.read_text().splitlines():
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    action = event.get("Action")
    if action not in ("pass", "fail", "skip"):
        continue
    name = event.get("Test")
    if name:
        tests[action] += 1
        if "/" not in name:
            top[action] += 1
    else:
        packages[action] += 1
print(f"Top-level tests: PASS={top['pass']} FAIL={top['fail']} SKIP={top['skip']}")
print(f"All test cases:  PASS={tests['pass']} FAIL={tests['fail']} SKIP={tests['skip']}")
print(f"Packages:       PASS={packages['pass']} FAIL={packages['fail']} SKIP={packages['skip']}")
if top['pass'] < 20 or tests['fail'] or tests['skip'] or packages['fail']:
    raise SystemExit(1)
