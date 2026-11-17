---
name: zeabur-catalog-dev
description: Development loop for the Zeabur Catalog repository — the checks CI runs, the cluster test tiers, and the rules for value comments and for manifests at scale. Use before changing any component, template, Go tool, Nix or CI file in this repo.
---

# Zeabur Catalog development loop

Data format and how to add a component or template: `docs/contribute/add-component.mdx`, `docs/contribute/add-template.mdx`. Template rules:
`docs/reference/template-standards.mdx`. Pull request rules: `CONTRIBUTING.md`.

## Checks (what CI runs)
Each pull request job is `nix develop -c scripts/ci/<job>.sh` (the jobs of `.github/workflows/ci.yml`).
The Release workflow builds its files with `scripts/ci/release.sh <version> <dir>`. Underneath:
```sh
nix develop -c go test ./...
nix develop -c go run ./tools/cmd/catalog validate   # schema, licences, references, value comments
nix develop -c go run ./tools/cmd/catalog schema     # writes schema/v1 from the Go types; edit those, not the JSON
nix develop -c go run ./tools/cmd/catalog api -version 0.0.0   # the static API into out/api, checked against the schemas
nix develop -c go run ./tools/cmd/catalog render     # helm template every profile, kubeconform
nix develop -c go run ./tools/cmd/catalog template lint -values values.json <id>   # safety rules, kubeconform
```
Cluster tests come in tiers (`docs/contribute/tests.mdx`). `check` starts its own etcd and kube-apiserver and
needs no Docker. `smoke` and `full` create their own kind clusters; run them on a disposable Linux machine or VM:
```sh
nix develop -c go run ./tools/cmd/catalog e2e -tier check            # every pull request
nix develop -c go run ./tools/cmd/catalog e2e -tier smoke <ids>      # merge queue: install, smoke
nix develop -c go run ./tools/cmd/catalog e2e -tier full <id>        # after a push to main; without IDs, what is stale
```
Without IDs, smoke tests everything. With `-since <commit>` it picks from the files changed between
that commit and `HEAD`, as the merge queue does (`tools/cmd/catalog/selection.go`; uncommitted
edits do not count): a changed component or template, with what installs it; a change under
`tools/`, `go.*`, `flake.*`, `config/` or `.github/workflows/`, the `smokeRepresentative` list in
`config/e2e.yaml`; a change to `config/kubernetes.yaml`, everything.

## Values and manifests

- A value comment states the constraint or the difference from the upstream default, not the
  history or the measurements behind it.
- One operator per cluster reconciles every instance of its kind. Never ship a controller,
  Deployment or CronJob per tenant item.
- A CronJob can run zero or two times, so its Job is idempotent. Keep finished Jobs bounded:
  `successfulJobsHistoryLimit: 3`, `failedJobsHistoryLimit: 1`, `ttlSecondsAfterFinished`.
- A workload that keeps required state in memory or on its own volume runs one replica, unless a
  shared store lets it run more.
- A value that ends up in one object stays under the object size limits.
