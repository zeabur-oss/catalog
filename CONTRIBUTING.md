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
- Explain why a component is recommended in a comment at the top of its file, and
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

- **Every pull request (`check`, about 5 minutes):** Go formatting and tests, schema and license
  checks, rendering, template safety rules, and a dry run of every manifest
  against a real API server. Nothing is installed.
- **The merge queue (`smoke`, about 15 to 20 minutes):**
  what you changed, what it needs and what installs it is installed on one node and
  smoke tested. A change to a shared file such as `config/` or `tools/` tests the
  `smokeRepresentative` list in `config/e2e.yaml` instead of everything. A queued change that touches no component or template skips it.
- **Every Saturday and before a release (`full`):** everything on one and three
  nodes, amd64 and arm64, with idle memory and CPU, uninstall and leftover checks.
  A push to `main` that changes a component or template runs `full` for what is
  stale only: what has no passing run of its current file on a profile and
  architecture, and what installs it. `go run ./tools/cmd/catalog list -tier full -stale`
  prints that list.

Run the same tiers with `go run ./tools/cmd/catalog e2e -tier check`,
`-tier smoke <ids>` or `-tier full <id>`. `check` needs no Docker. `smoke` and `full`
create kind clusters on a Linux machine with Docker.
`nix develop -c scripts/ci/<job>.sh` runs one job of `.github/workflows/ci.yml` as CI does.

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

Choose the level by the effect on users: `major` when they have to change something, `minor` for
something new, `patch` for a fix (before 1.0, a `major` change makes the next minor
version, such as 0.2.0 after 0.1.2, and a `minor` change the next patch). A change only to CI or tooling needs
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
