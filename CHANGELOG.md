# Changelog

All notable changes to this catalog are documented in this file; each section comes from the change files in `.changeset/` (CONTRIBUTING.md, "Releases").

## 0.2.0 (2026-11-02)

### Breaking Changes

#### The catalog format drops the fields no client reads

Components lose `maturity`, `stability`, `alternatives`, `disruptsTraffic` and `upgradeNotes`; their `security` keeps only `summary`, `privileged` and `podSecurity`. Templates lose `metadata.icon`, `metadata.categories`, `scalability`, `upgradeNotes` and `services[].baselineJustification`. `index.json` no longer has `nonOSILicenses`, `capabilities`, `budget` or `templates`. Components and templates also lose `metadata.description`; its text is now a comment at the top of the file. A file with one of these fields fails validation.

Before 1.0, a minor release can change the file format, so pin an exact release.

## 0.1.3 (2026-11-01)

### Features

#### Components can list the ids they replace, and `conformance/select/` gives cases for how installers choose components

`replaces` on a component lists the ids of removed components it took over. `builds/zeabur-builder` replaces `container-registry/zot`. When `enable` or `disable` names an old id, the installer stops and names the new id. The new reference page "How installers choose and install components" gives the rules, and `bundle.tar.gz` now has `conformance/select/`, with one case per rule in the format of `schema/v1/select-case.schema.json`.

#### `conformance/template/` gives cases for how clients render templates

The new reference page "How clients render templates" gives the checks before rendering, the labels every object gets, how a client tells whether what a template requires is installed, and which language it shows. `bundle.tar.gz` now has `conformance/template/`, with one case per rule in the format of `schema/v1/template-case.schema.json`. `catalog template render` names each failed check by a code, and a missing cluster issuer is now its own error.

#### Templates have a one-sentence summary in each language

`metadata.summary` holds the summary in English and `metadata.locales.<zh-TW|zh-CN>.summary` in Chinese, as for components. `index.json` carries both, so Zeabur no longer keeps its own copy.

### Fixes

#### A personal install leaves VictoriaMetrics and VictoriaLogs off machines with less than about 1.7 GiB of memory

`victoria-metrics` and `victoria-logs` give `resources.workMemoryMiB`, measured on KubeSolo under load. The VictoriaMetrics operator is now optional on `single-node`, so it is installed only with them.

## 0.1.2 (2026-11-01)

### Features

- `builds/zeabur-builder` installs Zeabur Builder 0.1.1 together with the zot registry it pushes to, so the two are added and removed as one component

#### A component's `health` lists read-only checks that an installer can run on a cluster

Each is one `kubectl wait`, with a timeout and advice for when it fails. e2e runs them after install. Longhorn, cert-manager, Traefik and metrics-server have them.

#### The `k3s` and `rke2` e2e profiles install k3s v1.36.5+k3s1 and RKE2 v1.36.5+rke2r1 on the runner

These are the releases Zeabur Installer pins. Their measurements are keyed `k3s-<architecture>-<version>` and `rke2-<architecture>-<version>`.

#### `container-registry/zot` is removed

zot only serves Builder's images and now comes with `builds/zeabur-builder`.

#### `index.json` gives each component `testedOn`, the distributions among k3s, RKE2 and KubeSolo where its e2e test passed at the pinned version

A failed run marks its environment `failed` in `measurements/`. The distribution then stays out of `testedOn` until a run passes again, and `measured` in the index leaves the environment out.

### Fixes

- The hand-written `capabilities` list of the `kubesolo` block in `config/kubernetes.yaml` is removed
- The `kubesolo` profile pins KubeSolo v1.2.3, which keeps a Job's `nodeSelector`, and tests the `databases` capability
- KubeSolo is removed from Longhorn's `distributions`
- Traefik's smoke check of its GatewayClass is now a health check

#### The `kubesolo` profile, like `k3s` and `rke2`, tests every component that single-node supports and whose `distributions` include it

`distributions` now lists the distributions to test a component on, and `testedOn` the ones where that test passed.

#### `databases/mariadb-operator` turns off client-go's streaming list

On a new KubeSolo cluster its controllers started too late to sync their caches, so a MariaDB object was never reconciled.

## 0.1.1 (2026-10-28)

The first release. zeabur-oss repositories start at 0.1.1; there is no 0.1.0.

### Added

- Releases carry `SHA256SUMS` and a GitHub build provenance attestation for every file. zeabur-oss/.github's shared release workflow signs them, so the signatures name that workflow and `zeabur-oss/catalog` as the repository (docs/reference/releases.mdx).
- `security.podSecurity: baseline` names a component whose namespaces cannot run under restricted. K8up and MOCO set it.
- Component releases pin their content: `chartDigest` for charts, `urlDigests` for URL manifests. `catalog render` checks both before it renders.
- `catalog render` fails on a component image without `@sha256:`, as the template lint already does. `unpinnedImages` lists the images a chart cannot pin, with the reason.
- Velero: the `file-backup` input installs the node agent. It is off by default, so the velero namespace needs no privileged Pod Security level. Turn it on for Backups that copy volume files, such as Zeabur's backup schedule.
- Traefik keeps `ClientHost` in its access log, so the log holds the client address.
- `manifests.yaml` allows only field substitution. `catalog validate` rejects conditions, functions, pipelines and trim markers. Inputs take a `default`. Ghost has the input `smtpSecure`, and Dify has `storageCAFile` for a private storage CA.
- `catalog crosscheck` writes `crosscheck.json`, the rendered objects of every template with its e2e values, into the release bundle.
- Every component chart, URL manifest and image is pinned by digest.
- Renovate updates image and OCI chart digests together with their tags.
- The security summaries of Velero, the VictoriaMetrics operator and Longhorn say which tokens equal cluster-admin.

