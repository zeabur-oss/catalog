# Contributing

Thanks for helping. The README explains the data format and how to add an
upstream; this file covers the process.

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

- One upstream per pull request when you can; CI tests each component in
  isolation.
- Explain why the recommendation is right in the file's `rationale`, and why
  each value is set in a comment next to it.
- Never copy secrets, account IDs, domains, IP addresses or cloud-specific
  settings into values.
- Do not edit `measurements/`; CI writes it.
- Run `go run ./tools/cmd/catalog validate` and `go run ./tools/cmd/catalog render -kubeconform`
  before pushing (inside `nix develop`).

## License

By contributing you agree that your contributions are licensed under the
Apache License 2.0.
