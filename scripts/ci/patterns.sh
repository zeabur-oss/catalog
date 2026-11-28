#!/usr/bin/env bash
# Patterns that clippy cannot see. Each rule fails CI when its pattern matches, names the right way
# in its reason, and carries an example that it must catch: the self-test runs first, so a rule
# that stops matching fails too.
set -euo pipefail
cd "$(dirname "$0")/../.."

# name | files (rg glob) | pattern (rg -U, may hold |) | reason | an example that breaks the rule
rules=(
  "string-problems|crates/**/*.rs|fn \w*problems?\w*(<[^>]*>)?\([^)]*\)\s*->[^{;]*Vec<String>|a check returns its own problem enum, wrapped by zeabur_catalog_format::problem::ProblemKind|fn license_problems(x: &str) -> Vec<String> {"
  "string-problems-out|crates/**/*.rs|fn \w*problems?\w*(<[^>]*>)?\([^)]*&mut Vec<String>|a check pushes its own problem enum into zeabur_catalog_format::problem::Problems|fn walk_problems(\n    raw: &Value,\n    out: &mut Vec<String>,\n) {"
  "problem-text|crates/**/*.rs|\bto_string\(\)\s*\.(contains|starts_with|ends_with)\(|a test matches a problem by its kind, file and path, not by the text Display builds|        assert!(found[0].to_string().contains(\"zot\"));"
  "text-tools|scripts/**|\b(grep|sed|awk|cut)\b|scripts read JSON with jq, YAML with yq, objects with kubectl -o jsonpath and search with rg|ready=\$(cut -d, -f2 pods.csv)"
  "registry-ca-copy|{components,templates,docs,crates}/**|\bca(From|Bundle)\b|pass the source Secret as registry.caSecretRef to Zeabur Builder, and name its published CA in provides.registry.ca|      caFrom: {namespace: zot, secret: zot-tls}"
)

failed=0
example_dir=$(mktemp -d)
trap 'rm -rf "$example_dir"' EXIT
for rule in "${rules[@]}"; do
  IFS='|' read -r -a fields <<<"$rule"
  name=${fields[0]} glob=${fields[1]} reason=${fields[-2]} example=${fields[-1]}
  pattern=$(IFS="|"; echo "${fields[*]:2:${#fields[@]}-4}")
  printf '%b\n' "$example" >"$example_dir/example.rs"
  if ! rg -q -U "$pattern" "$example_dir/example.rs"; then
    echo "patterns.sh: rule $name does not catch its own example" >&2
    failed=1
  fi
  if rg -n -U --glob "$glob" --glob '!scripts/ci/patterns.sh' "$pattern" .; then
    echo "patterns.sh: $name: $reason" >&2
    failed=1
  fi
done
exit "$failed"
