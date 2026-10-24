---
name: zeabur-catalog-dev
description: Development loop for the Zeabur Catalog repository — the checks CI runs and the engineering standards for comments, commits, tests, design and scale. Use before changing any component, template, Go tool, Nix or CI file in this repo.
---

# Zeabur Catalog development loop

Data format and how to add a component or template: `README.md`. Template rules:
`docs/template-standards.md`. Pull request rules: `CONTRIBUTING.md`.

## Checks (what CI runs)
```sh
nix develop -c go test ./...
nix develop -c go run ./tools/cmd/catalog validate   # schema, licences, references, value comments
nix develop -c go run ./tools/cmd/catalog index
nix develop -c go run ./tools/cmd/catalog render     # helm template every profile, kubeconform
nix develop -c go run ./tools/cmd/catalog template lint -values values.json <id>   # safety rules, kubeconform
```
Cluster tests come in tiers (README, "CI"). Run them on a Linux machine with Docker, never
on a kubeconfig context you did not create; each tier creates its own kind clusters:
```sh
nix develop -c go run ./tools/cmd/catalog e2e -tier l0               # every pull request
nix develop -c go run ./tools/cmd/catalog e2e -tier l1 <ids>         # merge queue: install, smoke
nix develop -c go run ./tools/cmd/catalog e2e -tier l2 -profile single-node <id>   # nightly: full
```

## Standards
- Writing a comment, doc comment or commit message; adding or removing a test; adding an
  abstraction, option or flag; or code that lists, polls, stores or fans out: apply
  [`STANDARDS.md`](STANDARDS.md) and check the change against it.
