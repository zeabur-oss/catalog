---
default: minor
---

# Measurements name each environment by its fields

`measured` in `index.json` and `environments` in each measurement file are lists. Each entry names its environment with `distribution`, `profile`, `platform` (`os` and `architecture`), `kubernetesVersion` and, for a distribution on the host, `distributionVersion`. A measurement file gives each run a `result`, `passed` or `failed`, and a reader treats any other value as not passed. `versions` lists `{release, version}` for a component and `{image, version, digest}` for a template. `testedOn` lists `{distribution, platforms}`, with each platform where the test of the pinned release passed, so a failure on arm64 no longer hides a pass on amd64. Component measurements move to `measurements/components/<capability>/<upstream>.json`. The files in the old layout are removed, and the next nightly run measures every component again. An untested component is no longer left out: a default that `testedOn` doesn't list for the distribution and platform is installed with an `untested` warning, as an enabled one is, and the `untested` reason and the `needs-untested` error are removed.
