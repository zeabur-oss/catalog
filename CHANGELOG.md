# Changelog

All notable changes to this catalog are documented in this file; each section comes from the change files in `.changeset/` (CONTRIBUTING.md, "Releases").

## 0.4.0 (2026-11-06)

### Breaking Changes

#### Switching profiles keeps zot and VictoriaMetrics running

zot's volume is 20 GiB and VictoriaMetrics' 5 GiB on both `single-node` and `multi-node`, the sizes `single-node` already had. Before, a switch changed the size: Kubernetes refused to update zot's StatefulSet, so its upgrade failed, and the VictoriaMetrics operator couldn't resize the `local-path` volume, so it stopped updating vmsingle and left it on the `single-node` memory limit. `local-path` ignores the size. On Longhorn, a new `multi-node` install starts smaller than before; grow the claim with `kubectl edit pvc` when it fills.

An install created on `multi-node` has a 50 GiB zot volume template and a 20 GiB VictoriaMetrics claim, so its first upgrade to this release needs three commands. Without them, zot's upgrade fails and the operator stops updating vmsingle. Run them, then `apply` again:

```sh
kubectl -n zot delete statefulset zot --cascade=orphan
kubectl -n observability annotate pvc vmsingle-vm-victoria-metrics-k8s-stack operator.victoriametrics.com/pvc-allow-volume-expansion=false
kubectl -n observability rollout restart deployment victoria-metrics-operator
```

The first keeps zot's Pod and volume, and the upgrade creates the StatefulSet again with the new template. The second tells the operator to leave the existing claim's size alone, and the third makes it try vmsingle again, which it does not do on its own until the VMSingle changes again.

### Fixes

#### CrowdSec agents come back after their node restarts

A restarted agent Pod keeps its name, and the LAPI refused to register that name again, so the agent stayed down until someone ran `cscli machines delete`. Agents now sign in with a client certificate from cert-manager, and the Traefik bouncer reaches the LAPI over HTTPS. CrowdSec now requires cert-manager (capability `certificates`). Its certificates last ten years, because agents don't reload a renewed certificate. The LAPI deletes a certificate machine after seven days without a heartbeat, and token auto-registration is off.

Upgrading removes the LAPI's `crowdsec-config-pvc` volume, which only kept the old password credentials. Decisions and bouncer registrations stay on the `data` volume, and agents that signed in with a password show as stale in `cscli machines list` until they are deleted.

#### A changed component is no longer listed in testedOn until a run of the new file passes

Each measured environment records `sourceDigest`, a digest of the component file, or of `template.yaml` and `manifests.yaml` for a template. `testedOn` counts only passing runs whose digest equals the current file's. Runs without the field no longer count, so every component reads as untested until the next weekly run measures it again.

## 0.3.0 (2026-11-04)

### Breaking Changes

#### Components no longer have `replaces`

`builds/zeabur-builder` no longer lists `container-registry/zot`, `index.json` and `schema/v1/component.schema.json` no longer have `replaces`, and `conformance/select/` no longer has the `replaced` error. An installer reports `container-registry/zot` in `enable` or `disable` as unknown. An installer no longer knows that Zeabur Builder's zot is the old component, so on a server that still has `container-registry/zot`, remove it before you install Builder. With zeabur-installer, run `kubectl -n kube-system delete helmchart zot`, wait until `kubectl -n zot get pods` lists none, then `kubectl delete namespace zot`. Builds push their images again.

### Features

#### References, release types and Pod Security are explicit fields

Components and templates name what they need with objects: `requires` entries are `{capability}` or `{component}`, and a contribution's `target` is `{component, release}`. Every release has a `type` (`helm`, `manifest` or `inline`). `security.podSecurity` replaces `security.privileged`, and template services use `podSecurity` in place of `securityProfile`. Every component lists its `distributions` and a `kubeVersion` range, and `detection.addons` replaces `detection.k3sAddons` with an entry for each distribution, such as `rke2-traefik` for RKE2. Distributions that e2e does not test (k0s, MicroK8s, kubeadm) are no longer accepted, and Longhorn no longer lists them.

#### The index names its release, and locales take any language

`index.json` has `catalog` with the `version` of the release that built it. The keys of every `locales` map are BCP 47 tags, and `config/locales.yaml` lists the languages each map must have, now `zh-TW` and `zh-CN`. The release docs give the rule for each value a reader doesn't know, such as an unknown release `type` or `generate.type`.

#### Generated inputs state their length and encoding

`generate` on a component or template input is an object: `{type: random, bytes, encoding}` or `{type: clusterIP}`. `bytes` and `encoding` are required, so components and templates no longer have different hidden defaults (64 hex and 32 base64url). `generateBytes` and `generateEncoding` are removed. Input names are camelCase on components too, such as `clusterIp` on Zeabur Builder and `s3Url` on Velero.

#### Measurements name each environment by its fields

`measured` in `index.json` and `environments` in each measurement file are lists. Each entry names its environment with `distribution`, `profile`, `platform` (`os` and `architecture`), `kubernetesVersion` and, for a distribution on the host, `distributionVersion`. A measurement file gives each run a `result`, `passed` or `failed`, and a reader treats any other value as not passed. `versions` lists `{release, version}` for a component and `{image, version, digest}` for a template. `testedOn` lists `{distribution, platforms}`, with each platform where the test of the pinned release passed, so a failure on arm64 no longer hides a pass on amd64. Component measurements move to `measurements/components/<capability>/<upstream>.json`. The files in the old layout are removed, and the next weekly run measures every component again. An untested component is no longer left out: a default that `testedOn` doesn't list for the distribution and platform is installed with an `untested` warning, as an enabled one is, and the `untested` reason and the `needs-untested` error are removed.

#### The index publishes each component's memory requests, measured in the full tier

`measured.<env>.requests.memoryMiB` is the sum of the memory requests of the pods a component runs, read from the cluster after the full tier settles. Jobs and finished pods are left out.

#### System Upgrade Controller is no longer installed by default

`node-updates/system-upgrade-controller` is now optional on both profiles. Nothing installs a Plan for it, and zeabur-installer upgrades k3s and RKE2 itself: `apply` puts back the version it pins on a node that a Plan upgraded. Install it for Plans that leave the Kubernetes version alone, such as OS package upgrades.

#### Resources are amounts of each resource, and the index moves to version 3

`resources.work` on a component lists each kind of work it does. An entry gives `requests`, `limits` and `use` in Kubernetes quantities of `cpu` and `memory`, such as `use: {memory: 500Mi}`, and `use` replaces `resources.workMemoryMiB`. `index.json` publishes each as an object with `memoryMiB` and `cpuMillis`, so a resource added later is one more field in that object. `measured.<env>` moves to the same shape: `idle` replaces `idleMemoryMiB` and `idleCPUm`, and `requests` replaces `requestMemoryMiB`. An environment measured in the old format is left out of `index.json` until the next weekly run measures it again. `builds/zeabur-builder` publishes the requests and limits of a build pod, copied from `buildkitd.resources` of its chart. The controller creates that pod, and no render shows it. An installer adds the build pod's `requests.memoryMiB` to the measured `requests.memoryMiB` when it counts the memory a team needs. Zeabur Builder moves to chart 0.1.2-beta.1, which sets that value.

### Fixes

#### Single-node memory requests follow what the default components use

Most default components ask for less memory on one node, such as Zeabur Builder's controller, metrics-server, CloudNativePG and the VictoriaMetrics operator's config reloaders. VictoriaMetrics, vmagent, vmalert, kube-state-metrics, cert-manager, the VictoriaMetrics operator and zot ask for more, because they used more than they asked for with 35 pods running.

#### VictoriaLogs uses less memory and keeps the audit log

The log store limits its caches to 128 MiB and runs two searches at a time, and the collector keeps at most 512 MiB of lines for a log store that is down. On a single node VictoriaLogs works with 64 MiB over its idle use instead of 350 MiB, and its memory limits fall to 384 MiB and 128 MiB. The collector leaves out the logs of kube-system and observability, which only add noise to the 8 GiB the logs share, and now also keeps kube-apiserver's audit log as its own stream, so you can search who did what.

## 0.2.1-beta.0 (2026-11-03)

### Features

#### The index publishes each component's memory requests, measured in L2

`measured.<env>.requestMemoryMiB` is the sum of the memory requests of the pods a component runs, read from the cluster after L2 settles. Jobs and finished pods are left out.

### Fixes

#### Single-node memory requests follow what the default components use

Most default components ask for less memory on one node, such as Zeabur Builder's controller, metrics-server, CloudNativePG and the VictoriaMetrics operator's config reloaders. VictoriaMetrics, vmagent, vmalert, kube-state-metrics, cert-manager, the VictoriaMetrics operator and zot ask for more, because they used more than they asked for with 35 pods running.

#### VictoriaLogs uses less memory and keeps the audit log

The log store limits its caches to 128 MiB and runs two searches at a time, and the collector keeps at most 512 MiB of lines for a log store that is down. On a single node VictoriaLogs works with 64 MiB over its idle use instead of 350 MiB, and its memory limits fall to 384 MiB and 128 MiB. The collector leaves out the logs of kube-system and observability, which only add noise to the 8 GiB the logs share, and now also keeps kube-apiserver's audit log as its own stream, so you can search who did what.

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

