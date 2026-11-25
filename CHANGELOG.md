# Changelog

All notable changes to this catalog are documented in this file; each section comes from the change files in `.changeset/` (CONTRIBUTING.md, "Releases").

## 0.6.2 (2026-11-25)

### Features

#### API v1 has template measurements and READMEs

`v1/templates/<id>.json` has the template's `measurements`, as `v1/components/<id>.json` has the component's. Both have `readmes`, the locales of the READMEs that the API now publishes as Markdown files under `v1/components/<id>/` and `v1/templates/<id>/`. A reader no longer reads the repository for them.

#### Zeabur Builder follows a rotated zot CA without a reinstall

`provides.registry.ca` of `zeabur-builder` now names `builder-system/registry-ca`, a Secret that holds zot's CA and no key. Zeabur Builder keeps it in step with `zot/zot-tls`.

The component installs Zeabur Builder 0.3.0, whose registry values are `tokenSecretRef` and `caSecretRef`.

#### zeabur-catalog api writes the static API

`zeabur-catalog api --version <v> [--out <dir>]` writes `catalog.json`, `components/<id>.json`, `templates/<id>.json` and the schemas to `<dir>/v1/`, then checks every file against the schemas. The release builds the API with it instead of the Go harness. Its files hold the same data as before: the declarations and templates as written, the render of each profile at the newest Kubernetes version, the measurements, and requests, limits and storage as Kubernetes writes them.

#### zeabur-catalog check

Add `zeabur-catalog check`, which checks the component and template files against their schemas and that the components fit together.

#### zeabur-catalog check reads the repository policy

`zeabur-catalog check` also checks licences, the languages of every `locales` map, READMEs and `kubeVersion`. `zeabur-catalog-engine` has the `kubeVersion` reader that Zeabur used.

#### zeabur-catalog check reads the template rules

`zeabur-catalog check` checks each template's manifests, inputs, ID, services, version and requires. `zeabur-catalog-engine` refuses a template that uses an input it is not given or a domain no service routes.

#### zeabur-catalog check wants a comment on every Helm value

`zeabur-catalog check` reports each Helm value that a component sets without a comment saying why.

#### zeabur-catalog render

`zeabur-catalog render` renders each release of each component on each profile and Kubernetes version into `out/render/`, and checks the objects with kubeconform.

#### zeabur-catalog-engine checks pods against the Pod Security Standards

The new `pod_security` module of `zeabur-catalog-engine` runs the checks of Pod Security Admission at their latest version, with the same messages, and finds the pod of each workload kind.

#### Zeabur and the catalog render templates with the same code

Add `zeabur-catalog-engine` with the template renderer that Zeabur used.

#### An optional utoipa feature in zeabur-catalog-format

With the `utoipa` feature, `InputValue` implements `utoipa::ToSchema`, so a reader that builds its OpenAPI document with utoipa uses the type itself instead of a copy. The feature is off by default.

#### A link for every LicenseRef- licence

Component and template metadata have `licenseUrls`: the HTTPS URL of the text of each `LicenseRef-` licence in `license`, by its id. Every `LicenseRef-` id needs one, and the field names no id that `license` does not use. API v1 carries the field, so a client can link the licence. The n8n templates now spell their licence `LicenseRef-n8n-sustainable-use` in every template.

#### zeabur-catalog render checks pods, images and webhooks

`zeabur-catalog render` now runs the render rules that `catalog render` ran: every image has a digest unless `unpinnedImages` names it, a chart gives the same objects on two renders, and every pod meets the component's Pod Security level. The Pod Security checks come from `zeabur-catalog-engine`.

A component that serves an admission webhook must declare `health`, so that what needs the webhook installs only after it answers. `catalog render` is removed.

#### The schemas come from the Rust types, with TypeScript types

`schema/v1/` is now generated from the types of `crates/zeabur-catalog-format` by `cargo run -p zeabur-catalog -- schema`, which also writes `types/v1/catalog.ts`, the TypeScript types of the same files. The Go `catalog schema` command is gone.

Every file of the catalog passes them, as it passed the schemas before. Their `$defs` have new names, such as `Generate` for `generate`, so a `$ref` into another schema's `$defs` needs the new name. An internally tagged object, such as a release, now lists its fields in each variant of its `oneOf`.

A quantity in `resources.work` is a string, such as `cpu: "1"` or `memory: 256Mi`, as the Rust reader reads it. A number there no longer passes the schema. This changes a field's type inside v1, which is why this release is a new baseline: every published file already writes its quantities as strings.

CI no longer compares the text of the schemas with the latest release's. It checks that every file of the new API passes the latest release's schemas, with new fields and values allowed, and its own, because readers depend on the data. A schema change that no file of the catalog uses is not seen by the check. These schemas are the new baseline.

#### zeabur-catalog digest prints the source digest

`zeabur-catalog digest <dir>…` prints the `sourceDigest` of each component or template directory, from the new `digest` module of `zeabur-catalog-engine`. The digests are the same as before. The Go harness calls it, so the API files and the measurements share one definition.

#### The check's problems are typed

`zeabur-catalog-format` has `problem::Problem`: the file, the place in it as a `FieldPath` (written `health[0].condition`) and a `ProblemKind`, which wraps the enum of the check that found it. `check::from_yaml` takes the file's path and returns these, and `ReadError::Check` holds them. A reader matches on the kind instead of the text.

### Fixes

- Ghost, Halo, OpenList and Uptime Kuma declare their domain, so their Ingress gets a host

#### CI checks the catalog with zeabur-catalog check

`catalog validate` is removed. Run `cargo run -p zeabur-catalog -- check` instead.

#### zeabur-catalog compat checks the API against the published schemas

`zeabur-catalog compat <published> <api>` replaces the Go `catalog compat`. Every file of the new API must pass the schemas of the latest published release, which accept a field or a value they do not know, and its own schemas. The Go `catalog api` and `catalog compat` commands are removed.

#### zeabur-catalog-engine writes resource quantities as Kubernetes does

The new `quantity` module gives the canonical text of a resource quantity, as `resource.Quantity` of apimachinery v0.37.2 writes it: `0.5` becomes `500m`, `1024Mi` becomes `1Gi`, and a value already in canonical form keeps its text. The API files will write the requests and limits of a rendered workload with it.

#### licenses-update regenerates the licence list

`config/licenses.yaml` named a `licenses-update` command that did not exist. `zeabur-catalog licenses-update` now writes the list: `curl -sSL https://spdx.org/licenses/licenses.json | cargo run -p zeabur-catalog -- licenses-update`. On SPDX License List 3.29.0 it writes the committed list.

#### A template memory writes kilo as lowercase `k`

`resources.memory` in a template takes the Kubernetes suffixes: `64k` passes and `64K`, which the API server refuses, fails `zeabur-catalog check`. `Pi`, `Ei`, `P` and `E` pass too.

## 0.6.1 (2026-11-22)

### Fixes

#### The API on `/v1/` only adds

Every release on `https://oss-catalog.zeabur.com/v1/` keeps each field and type of the last one and makes no field required, whatever its Zeabur Catalog version. A breaking change goes to `/v2/`. CI compares the schemas with those of the latest published release and fails on such a change.

#### Zeabur Builder installs chart 0.2.5

The upload Secrets of Zeabur Builder now hold the registry's CA in `ca.crt`, so an uploader that may read the Secret trusts zot without reading zot's own Secret. The controller may also read ServiceAccounts, which the optional agent builder needs.

The chart's ClusterBuilders name their type, with `spec.railpack` in place of `spec.planner`. A ClusterBuilder you made yourself needs the same change, and a Build's `spec.source` is now `type: git` with `git: {…}` or `type: archive` with `archive: {}`. Apply `crds/crds.yaml` of the chart before you upgrade, because Helm does not update CRDs.

#### Nodes pull Zeabur Builder's images without manual setup

On K3s, RKE2 and KubeSolo, the new `zot-node` DaemonSet points each node's container runtime at zot's fixed ClusterIP and gives it zot's CA, so a pod can pull `zot.zot.svc:5000/...` images right after you install the component. You no longer write `registries.yaml` by hand.

## 0.6.0 (2026-11-21)

### Breaking Changes

#### Each component has its own directory

A component moved from `components/<id>.yaml` to `components/<id>/component.yaml`, with its READMEs (`README.md`, `README.zh-TW.md`, `README.zh-CN.md`) in the same directory, as templates are. A reader of the release bundle reads the new paths. `catalog validate` refuses any other file in `components/`.

A component's `sourceDigest` is now the digest of its directory without the READMEs, so every component has a new digest and is untested until the next e2e run. Editing only a README of a component or a template keeps its `sourceDigest`.

### Features

#### Add the zeabur-catalog-format Rust crate

`crates/zeabur-catalog-format` holds the typed model of component, template, API and measurement files and the rules installers follow: install order, choosing components, contributions and release values. Readers of the API ignore fields and values they do not know, and the catalog's check refuses them by path. A test runs every `conformance/select/` case against it.

### Fixes

#### A component that adds values to another needs a Helm release

`catalog check` now refuses `contributes` from a component without a `helm` release. Installers find the components they installed by their Helm releases, so they could not add the values of such a component again when they write the target. CrowdSec and VictoriaMetrics already have one.

#### Release downloads retry transient failures

`catalog check` and `catalog cluster` now retry a release download that fails with a connection error, a 5xx or a 429 response, up to four attempts with backoff. A 4xx response and a checksum mismatch still fail at once.

## 0.5.6 (2026-11-20)

### Features

#### Components have READMEs

Each component has `components/<id>.README.md`, `.README.zh-TW.md` and `.README.zh-CN.md` with an Introduction, Use it and Remove it, and optional Before you install and Troubleshooting sections. Use it links to the Zeabur docs page where the component shows up. `catalog validate` checks the files and headings, and requires Before you install when an input needs a value from outside the cluster.

#### Templates reach each other's private services through a client label

A NetworkPolicy peer may now select the template's clients, Pods with the label `templates.zeabur.com/<id>-client: "true"`, as well as the template's own Pods. The OpenClaw Browser Sandbox lets in Pods with `templates.zeabur.com/openclaw-browser-sandbox-client: "true"` and drops the `clientApp` input. The OpenClaw template sets that label, so OpenClaw can now reach the sandbox. Ollama lets in any Pod with `templates.zeabur.com/ollama-client: "true"`, as its security notes say.

#### Templates have READMEs and link their docs

`metadata.docs` links the upstream usage docs. Each template has `README.md`, `README.zh-TW.md` and `README.zh-CN.md` with an Introduction, numbered Getting started steps (where to open the app, how the first person signs in, which Secret holds each generated value), and optional Configuration and Troubleshooting sections. `catalog validate` checks that the files and these headings exist and that Getting started names the Secret of every `generate` input.

#### Velero trusts a store's own CA

Velero has a new input, `caCert`, for the CA certificate of an S3 store that serves HTTPS with its own certificate, such as a self-hosted MinIO. Leave it empty for a store with a public certificate. The `s3Url` description now says the URL must use https, because the store encrypts each backup with the backup key and S3 servers refuse that encryption over plain HTTP.

### Fixes

#### Longhorn's node check names no installer

When a node isn't ready for Longhorn, the advice says to install what the node lacks, such as open-iscsi, an NFS client or a kernel module. It no longer tells you to run zeabur-installer, which Zeabur removed.

#### VictoriaMetrics and VictoriaLogs carry no Zeabur rules

Zeabur's notification rules and its log access move to Zeabur, which applies them when it installs VictoriaMetrics and VictoriaLogs. They ship with the Zeabur version that uses them.

- VictoriaMetrics no longer installs the `zeabur-notifications` VMRule. Zeabur applies a VMRule of the same name, so a cluster that Zeabur manages needs no step. Without Zeabur, delete the old one: `kubectl -n observability delete vmrule zeabur-notifications`.
- The custom resource metrics are named `kube_customresource_*` instead of `zeabur_*`, such as `kube_customresource_cnpg_backup_created`.
- kube-state-metrics exports every label of Jobs and namespaces, not only Zeabur's.
- The VictoriaLogs NetworkPolicy no longer admits Zeabur Proxy. Zeabur adds its own policy for it.

## 0.5.5 (2026-11-19)

### Fixes

#### Zeabur Builder 0.2.3

The `builder` component installs chart 0.2.3. Builds can be capped per cluster, run on build nodes set aside by a taint, and record their steps and kstatus conditions. Build Jobs get a memory limit, and the images are signed by Builder's own release workflow.

## 0.5.4 (2026-11-18)

### Fixes

#### The README and the docs start from Zeabur users, in English, Traditional Chinese and Simplified Chinese

The README comes in English, Traditional Chinese and Simplified Chinese, each with the new cover in its language. It and the docs home page first explain what the catalog does for people who use it through Zeabur, then how to use it on its own. A new page, Templates from zeabur.com, gives the catalog template for each popular zeabur.com template. Each step of Deploy a template now has a command that checks it worked, with the expected output.

## 0.5.3 (2026-11-18)

### Fixes

#### v0.5.3 has the release files that v0.5.2 lacks

The v0.5.2 release was published before the Release workflow added its files, and a published release takes no more files. Use v0.5.3: its catalog is the same as the one v0.5.2 was meant to carry.

## 0.5.2 (2026-11-18)

### Features

#### The API publishes the templates

`catalog api` writes `v1/templates/<id>.json` for each template: its `id` and `sourceDigest`, its `template` (`template.yaml` as JSON) and its `manifests` (the text of `manifests.yaml`). `v1/catalog.json` lists each template's `id` and `sourceDigest` in `templates`, and `v1/schema/` holds `template.schema.json` and `template-file.schema.json`.

#### Releases are signed by the catalog's own release workflow, and checksums.txt is gone

Verify `SHA256SUMS` with `--certificate-identity "https://github.com/zeabur-oss/catalog/.github/workflows/release.yml@refs/tags/<tag>"`, and attestations with `--signer-workflow zeabur-oss/catalog/.github/workflows/release.yml` ([releases](docs/reference/releases.mdx)). Releases up to v0.5.1 keep the identity of `zeabur-oss/.github`. `SHA256SUMS` already lists the two archives, so a release no longer has `checksums.txt`.

### Fixes

- A weekly run that measures one item updates the right file

## 0.5.1 (2026-11-13)

### Features

#### Measurements record the digests of the components a run installed first

Each measurement in `v1/components/<id>.json` has `dependencies`, the `sourceDigest` of each component that the run installed before this one, by ID. Measurements whose inputs differ from the release's files and pinned Kubernetes and distribution releases are no longer published: a run of older files, of another `kubernetesVersion` on kind, or of another `distributionVersion` on a host distribution.

### Fixes

- The crash loop alert fires on restarts

#### Memos moves to 26.11.0

The Memos template now creates the administrator and turns off sign-up from the files in `/etc/memos/config`, because Memos 26.10.0 and later no longer read `/etc/secrets` and have removed the `/api/v1` routes.

## 0.5.0 (2026-11-11)

### Breaking Changes

#### Component IDs are flat, and each component declares its capability in `provides`

A component is `components/<id>.yaml`, and its ID is the file name: `traefik`, not `ingress/traefik`. Its capability is `provides.capability`, such as `provides: {capability: ingress}`, and is no longer read from a folder. `index.json` drops the `capability` field of each component; read `provides.capability` instead. `requires` entries are `{component: <id>}` or `{capability: <capability>}` as before. Measurements move to `measurements/components/<id>.json`.

An installer reads a name in `enable` or `disable` as a component ID first, then as a capability. `catalog validate` fails when a capability is the ID of a component that offers another capability, and when a template has the ID of a component.

`indexVersion` is now 4. `testedOn` is empty for every component until the next full run measures the moved files again.

#### The index publishes idle usage per workload pod

Each entry of `measured` in `index.json` has `pods` instead of `idle` and `requests`. A row is a workload with its `kind` and the `memory` and `cpu` of one pod of it, the median over its pods, as Kubernetes quantities such as `22Mi` and `3m`. Totals are not stored. A reader multiplies each row by the replicas of the objects the component renders and reads requests from them too. The measurements are regenerated by the next full run.

`indexVersion` is now 4.

#### A static API replaces index.json

Each release publishes `api-v1.tar.gz`, and each release that isn't a pre-release publishes the same files at `https://oss-catalog.zeabur.com/v1/`. `v1/catalog.json` lists the release, the profiles, the Kubernetes versions, the pinned distributions and each component's `sourceDigest`. `v1/components/<id>.json` holds a component's declaration as written, what it renders for each profile it supports, with the replicas, requests, limits and storage of its workloads and claims, and its measurements. Each file passes its schema in `v1/schema/`, and the Go types of the catalog's tools generate those schemas.

`index.json` and `indexVersion` are gone: the release no longer has `index.json`, and `bundle.tar.gz` no longer carries it. The API holds facts only: `testedOn`, `installOrder` and the rounded `resources.work` are gone, and a reader works them out as the installer reference describes. A measurement keeps its `result`, including a failed run.

`schema/v1/component.schema.json` and `schema/v1/template.schema.json` are generated too. A template's localized `summary` must be a single line, like a component's, and a template input generates `random` values only.

#### Health checks name their objects in fields

A `health` check no longer holds `kubectl wait` arguments in `wait`. It names its objects in `resource`, with a `kind`, a `namespace` when the kind has one, and exactly one of `name`, `selector` or `all: true`, and the condition they reach in `condition`. `catalog validate` fails on a kind the schema doesn't list and on a missing field. An installer builds the `kubectl wait` call from these fields, as the installer reference describes. cert-manager's health check is now three checks, one for each Deployment.

## 0.4.2 (2026-11-08)

### Fixes

#### Zeabur Builder moves to chart 0.2.1

buildkitd now always runs in a user namespace, and a build starts again after the buildkitd cache size or storage class changes. The component's `kubeVersion` (1.34 or newer) already covers the user namespaces this needs.

#### metrics-server answers from every node

On a `multi-node` cluster whose `kube-system` lets in only its own pods, as K3s's CIS hardening does, kube-apiserver could not reach the metrics-server replica on another node. Half of the `metrics.k8s.io` reads waited about a minute and then failed, so `kubectl top` and Zeabur's Nodes page hung. The component now has a NetworkPolicy that lets requests into metrics-server's port, as K3s's hardening guide does for its own metrics-server.

#### Each release is also at `https://oss-catalog.zeabur.com/v1/`

Each release that is not a pre-release copies its signed `index.json`, `bundle.tar.gz`, `SHA256SUMS` and `SHA256SUMS.sigstore.json` to `https://oss-catalog.zeabur.com/v1/`, so Zeabur can fetch new and fixed templates without a new Zeabur release.

## 0.4.1 (2026-11-07)

### Fixes

#### Operators on `multi-node` request what they use

On `multi-node`, each replica of the CloudNativePG, Dragonfly, MariaDB, MOCO, Percona MongoDB and SeaweedFS operators, Reloader and k8up now requests what it does on `single-node`, half or less of before. On a two-node cluster with apps, each leader uses 20–54 MiB, under its request. Their limits stay as they were, so they still have room as the objects they manage grow. The CloudNativePG Barman Cloud plugin requests 32 MiB on both profiles instead of 24 and 64, because it uses about 30 MiB on a cluster with apps.

#### VictoriaMetrics on `multi-node` requests as much memory as on `single-node`

`multi-node` now keeps the same metrics as `single-node`: node-exporter runs only the collectors the rules query, kube-state-metrics watches only the objects that map pods to workloads, and the API server's metrics and the kubelet's histograms are no longer collected. vmsingle requests 224 MiB instead of 512 MiB with a 1 GiB limit, and vmagent requests 128 MiB with a 512 MiB limit. vmalert and Alertmanager get the `single-node` requests too. The alert rules that read the API server's metrics go with them, as they already did on `single-node`. `multi-node` still keeps two weeks of history.

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

