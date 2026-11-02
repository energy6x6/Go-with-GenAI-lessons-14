#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export ARTIFACTS_DIR="${ARTIFACTS_DIR:-../lesson14-artifacts}"
output_dir="$(python3 - <<'PY'
import os
from pathlib import Path
root = Path.cwd().resolve()
out = Path(os.environ['ARTIFACTS_DIR']).resolve()
if out == root or root in out.parents:
    raise SystemExit('ARTIFACTS_DIR must be outside the repository')
out.mkdir(parents=True, exist_ok=True)
print(out)
PY
)"
test -z "$(gofmt -l cmd internal docs web migrations)"
go mod tidy
git diff --exit-code -- go.mod go.sum
go build ./...
go vet ./...
go test -race -count=1 -timeout=5m -coverprofile="$output_dir/coverage.out" -json ./... > "$output_dir/tests.jsonl"
python3 scripts/test_summary.py "$output_dir/tests.jsonl" | tee "$output_dir/test-summary.txt"
go tool cover -func="$output_dir/coverage.out" | tail -n 1 | tee -a "$output_dir/test-summary.txt"
echo "Evidence: $output_dir"
