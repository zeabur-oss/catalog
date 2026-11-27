---
default: minor
---

# `zeabur-catalog` renders templates for deployers and runs the dry run

`zeabur-catalog template render` and `zeabur-catalog template inputs` render a template and its generated Secrets from a release's static API (`--api <dir>/v1`). `zeabur-catalog crosscheck` writes `crosscheck.json`, and `zeabur-catalog dry-run` sends every render to a local kube-apiserver with `--dry-run=server` and checks profile switches and upgrades from the base. The Go `catalog template` and `catalog crosscheck` commands are removed; `catalog e2e -tier check` now calls the Rust commands.
