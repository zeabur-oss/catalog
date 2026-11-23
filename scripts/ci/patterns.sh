#!/usr/bin/env bash
# Patterns that clippy cannot see. Each rule fails CI when its pattern matches, names the right way
# in its reason, and carries an example that it must catch: the self-test runs first, so a rule
# that stops matching fails too.
set -euo pipefail
cd "$(dirname "$0")/../.."

# name | files (rg glob) | pattern (rg -U) | reason | an example that breaks the rule
rules=(
  "string-problems|crates/**/*.rs|fn \w*problems?\w*(<[^>]*>)?\([^)]*\)\s*->[^{;]*Vec<String>|a check returns its own problem enum, wrapped by zeabur_catalog_format::problem::ProblemKind|fn license_problems(x: &str) -> Vec<String> {"
  "string-problems-out|crates/**/*.rs|fn \w*problems?\w*(<[^>]*>)?\([^)]*&mut Vec<String>|a check pushes its own problem enum into zeabur_catalog_format::problem::Problems|fn walk_problems(\n    raw: &Value,\n    out: &mut Vec<String>,\n) {"
)

failed=0
example_dir=$(mktemp -d)
trap 'rm -rf "$example_dir"' EXIT
for rule in "${rules[@]}"; do
  IFS='|' read -r name glob pattern reason example <<<"$rule"
  printf '%b\n' "$example" >"$example_dir/example.rs"
  if ! rg -q -U "$pattern" "$example_dir/example.rs"; then
    echo "patterns.sh: rule $name does not catch its own example" >&2
    failed=1
  fi
  if rg -n -U --glob "$glob" "$pattern" crates; then
    echo "patterns.sh: $name: $reason" >&2
    failed=1
  fi
done
exit "$failed"
