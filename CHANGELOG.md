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

### Changed

- Every component chart, URL manifest and image is pinned by digest.
- Renovate updates image and OCI chart digests together with their tags.
