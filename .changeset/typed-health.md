---
default: major
---

# Health checks name their objects in fields

A `health` check no longer holds `kubectl wait` arguments in `wait`. It names its objects in `resource`, with a `kind`, a `namespace` when the kind has one, and exactly one of `name`, `selector` or `all: true`, and the condition they reach in `condition`. `catalog validate` fails on a kind the schema doesn't list and on a missing field. An installer builds the `kubectl wait` call from these fields, as the installer reference describes. cert-manager's health check is now three checks, one for each Deployment.
