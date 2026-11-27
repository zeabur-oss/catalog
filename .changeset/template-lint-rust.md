---
default: patch
---

# `zeabur-catalog render` checks each template's rendered objects

It renders every template with its e2e values, checks the objects against the template rules (namespace, pinned images, resources, Pod Security, NetworkPolicies, operator resources and the services of `template.yaml`), and runs kubeconform against the CRDs of the components the template installs. The Go `catalog template lint` command is removed.
