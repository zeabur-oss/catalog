---
default: minor
---

# References, release types and Pod Security are explicit fields

Components and templates name what they need with objects: `requires` entries are `{capability}` or `{component}`, `replaces` entries are `{component}`, and a contribution's `target` is `{component, release}`. Every release has a `type` (`helm`, `manifest` or `inline`). `security.podSecurity` replaces `security.privileged`, and template services use `podSecurity` in place of `securityProfile`. Every component lists its `distributions` and a `kubeVersion` range, and `detection.addons` replaces `detection.k3sAddons` with an entry for each distribution, such as `rke2-traefik` for RKE2. Distributions that e2e does not test (k0s, MicroK8s, kubeadm) are no longer accepted, and Longhorn no longer lists them.
