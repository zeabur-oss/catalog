# Contributing

Thanks for helping. The README explains the data format and how to add a
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
  why each value is set in a comment next to it.
- A template meets the [template standards](docs/template-standards.md).
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
- **In the merge queue and after a merge (L1, about 15 to 20 minutes):** what
  you changed, and what installs it, is installed on one node and smoke
  tested.
- **Every night and before a release (L2):** everything on one and three
  nodes, amd64 and arm64, with upgrades from the last release, idle memory and
  CPU, uninstall and leftover checks.

Run the same tiers on a Linux machine with Docker:
`go run ./tools/cmd/catalog e2e -tier l0`, `-tier l1 <ids>`, or
`-tier l2 -profile single-node <id>`.

## License

By contributing you agree that your contributions are licensed under the
Apache License 2.0.
