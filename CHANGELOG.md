# Changelog

All notable changes to this catalog are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Component releases pin their content: `chartDigest` for charts, `urlDigests`
  for URL manifests. `catalog render` checks both before it renders.
- `catalog render` fails on a component image without `@sha256:`, as the
  template lint already does. `unpinnedImages` lists the images a chart cannot
  pin, with the reason.
- Velero: the `file-backup` input installs the node agent. It is off by default,
  so the velero namespace needs no privileged Pod Security level. Turn it on for
  Backups that copy volume files, such as Zeabur's backup schedule.

### Changed

- `manifests.yaml` allows only field substitution. `catalog validate` rejects
  conditions, functions, pipelines and trim markers, and `dnsEgress` is gone.
  Inputs take a `default`. Ghost has the new input `smtpSecure`, and Dify has
  `storageCAFile` for a private storage CA.
- `catalog crosscheck` writes `crosscheck.json`, the rendered objects of every
  template with its e2e values, into the release bundle.
- Every component chart, URL manifest and image is pinned by digest.
- Renovate updates image and OCI chart digests together with their tags.
- The security summaries of Velero, the VictoriaMetrics operator and Longhorn
  say which tokens equal cluster-admin.
