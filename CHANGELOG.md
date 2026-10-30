# Changelog

All notable changes to this catalog are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `builds/zeabur-builder` installs Zeabur Builder 0.1.1 together with the zot
  registry it pushes to, so the two are added and removed as one component.
- The `k3s` and `rke2` e2e profiles install k3s v1.36.5+k3s1 and RKE2
  v1.36.5+rke2r1, the releases the Zeabur installer pins, on the runner. Their
  measurements are keyed `k3s-<architecture>-<version>` and
  `rke2-<architecture>-<version>`.
- `index.json` gives each component `testedOn`: the distributions among k3s,
  RKE2 and KubeSolo where e2e passed at the pinned version. A failed run marks
  its environment `failed` in `measurements/`, which leaves the distribution
  out until it passes again, and `measured` in the index omits it.
- `health` lists read-only checks (one `kubectl wait` each, with a timeout and
  advice) that an installer can run on a cluster; e2e runs them after install.
  Longhorn, cert-manager, Traefik and metrics-server have them.

### Changed

- The `kubesolo` profile pins KubeSolo v1.2.3, which keeps a Job's `nodeSelector`,
  and tests the `databases` capability.
- The `kubesolo` profile, like `k3s` and `rke2`, tests every component that
  single-node supports and whose `distributions` allow it. `distributions` now
  states hard limits only.
- Traefik's smoke check of its GatewayClass and cert-manager's of the
  production issuer are now health checks.

### Removed

- `container-registry/zot`. zot only serves Builder's images and now comes with
  `builds/zeabur-builder`.
- The hand-written `capabilities` list of the `kubesolo` block in
  `config/kubernetes.yaml`.

## [0.1.1] - 2026-10-28

The first release. zeabur-oss repositories start at 0.1.1; there is no 0.1.0.

### Added

- Releases carry `SHA256SUMS` and a GitHub build provenance attestation for
  every file. zeabur-oss/.github's shared release workflow signs them, so the
  signatures name that workflow and `zeabur-oss/catalog` as the repository
  (docs/reference/releases.mdx).
- `security.podSecurity: baseline` names a component whose namespaces cannot run
  under restricted. K8up and MOCO set it.
- Component releases pin their content: `chartDigest` for charts, `urlDigests`
  for URL manifests. `catalog render` checks both before it renders.
- `catalog render` fails on a component image without `@sha256:`, as the
  template lint already does. `unpinnedImages` lists the images a chart cannot
  pin, with the reason.
- Velero: the `file-backup` input installs the node agent. It is off by default,
  so the velero namespace needs no privileged Pod Security level. Turn it on for
  Backups that copy volume files, such as Zeabur's backup schedule.
- Traefik keeps `ClientHost` in its access log, so the log holds the client address.
- `manifests.yaml` allows only field substitution. `catalog validate` rejects
  conditions, functions, pipelines and trim markers. Inputs take a `default`.
  Ghost has the input `smtpSecure`, and Dify has `storageCAFile` for a private
  storage CA.
- `catalog crosscheck` writes `crosscheck.json`, the rendered objects of every
  template with its e2e values, into the release bundle.
- Every component chart, URL manifest and image is pinned by digest.
- Renovate updates image and OCI chart digests together with their tags.
- The security summaries of Velero, the VictoriaMetrics operator and Longhorn
  say which tokens equal cluster-admin.

