# Contributing

Thanks for helping. The [documentation](docs/index.mdx) explains the data format and how to add a
component or a template; this file covers the process.

## Developer Certificate of Origin

This project uses the [Developer Certificate of Origin](https://developercertificate.org/)
(DCO) instead of a CLA. Every commit must carry a `Signed-off-by` line with the
author's name and email, which you add with `git commit -s`:

```text
Signed-off-by: Your Name <you@example.com>
```

By signing off you certify the DCO 1.1 for that commit. CI rejects pull
requests with commits that are not signed off by their author. To fix an
existing branch: `git rebase --signoff main`, then force-push.

## Pull requests

- One component or template per pull request when you can; CI tests each in
  isolation.
- Explain why a component is recommended in its `metadata.description`, and
  why each value is set in a comment next to it. Its `metadata.summary` says
  what it gives the cluster in one plain sentence, in English, zh-TW and zh-CN.
- A template meets the [template standards](docs/reference/template-standards.mdx).
- Never copy secrets, account IDs, domains, IP addresses or cloud-specific
  settings into values.
- Do not edit `measurements/`; CI writes it.
- Run `go run ./tools/cmd/catalog validate` and `go run ./tools/cmd/catalog render`
  before pushing, and `go run ./tools/cmd/catalog template lint <id>` for a
  template (inside `nix develop`).

## When tests run

- **Every pull request (L0, about 5 minutes):** Go tests, schema and license
  checks, rendering, template safety rules, and a dry run of every manifest
  against a real API server. Nothing is installed.
- **After a merge to `main` (L1, about 15 to 20 minutes):** what you changed,
  and what installs it, is installed on one node and smoke tested.
- **Every night and before a release (L2):** everything on one and three
  nodes, amd64 and arm64, with upgrades from the last release, idle memory and
  CPU, uninstall and leftover checks.

Run the same tiers with `go run ./tools/cmd/catalog e2e -tier l0`,
`-tier l1 <ids>` or `-tier l2 <id>`. L0 needs no Docker; L1 and L2 create kind
clusters on a Linux machine with Docker.
`scripts/ci.sh <job>` runs one pull request job as CI does, in the dev shell;
`scripts/ci.sh --list` lists them.

## Releases

The release workflow is the shared one of [zeabur-oss/.github](https://github.com/zeabur-oss/.github);
its stages are `bun scripts/release.ts check|build|publish|assets|verify`.

A pull request that changes what users of the catalog see (components, the index, the CLI, the
release files) adds a change file. `nix develop -c knope document-change` asks for its level and
summary and writes it to `.changeset/`; add paragraphs under the summary when it needs them:

```md
---
default: patch
---

# Longhorn's health check waits for the manager pods
```

Choose the level by the effect on users: `major` when they have to change something (`minor`
before 1.0), `minor` for something new, `patch` for a fix. A change only to CI or tooling needs
none; label the pull request `changelog: none` when its title is `feat`, `fix` or `perf` and users
see no difference. The Changeset check enforces this. Keep each paragraph on one line: the section
becomes the release body as it is.

To release:

1. Run the Release pull request workflow, with `prerelease: rc` for a release candidate. knope
   turns the change files into the next `CHANGELOG.md` section and sets `version` in `flake.nix`,
   and zeabur-oss-bot opens `chore(release): vX.Y.Z`. Edit `CHANGELOG.md` on that branch if the
   section should read differently.
2. Optionally rehearse: run the Release workflow by hand with the version. It builds the release
   and publishes nothing.
3. Merge the release pull request. zeabur-oss-bot tags the merge commit, and the Release workflow
   leaves a draft release; check it and publish it. If its verify job fails because of the check itself, fix
   the check and run `verify` again from a checkout, as zeabur-oss/.github's README shows.

## License

By contributing you agree that your contributions are licensed under the
Apache License 2.0.
