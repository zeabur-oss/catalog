<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/cover-dark.avif">
  <img alt="Zeabur Catalog: which chart, which version, which values, and why, for each Kubernetes component, installed and measured in CI" src="docs/images/cover-light.avif">
</picture>

# Zeabur Catalog

A tested catalog for Kubernetes clusters, in two parts:

- **Components** are what a cluster administrator installs once per cluster:
  ingress, certificates, metrics, database operators. For each capability the
  catalog says which upstream chart, which version, which values, and why.
- **Templates** are applications a project member installs into one namespace:
  n8n, Dify, WordPress, PostgreSQL. Each pins its images, sets
  resource requests and limits, runs under the Restricted Pod Security
  Standard where the application allows it, and says how it scales, how its
  data is backed up and what to check before an upgrade. A template names the
  components it needs in `requires`.

The catalog is data. CI installs and tests every component and template, and
each release publishes one signed bundle that any tool can read to generate
`helm install` commands, Flux or Argo CD manifests, k3s `HelmChart` objects or
template installs.

[Zeabur](https://github.com/zeabur-oss/zeabur) reads this catalog to suggest what
a cluster is missing; it never installs anything itself.

## Layout

```text
components/<capability>/<upstream>.yaml   one file per capability × upstream
templates/<id>/template.yaml              one application: inputs, services, security, scaling, smoke checks
templates/<id>/manifests.yaml             its Kubernetes objects
schema/v1/                                JSON Schemas for both
config/licenses.yaml                      OSI-approved SPDX identifiers; opt-in-only exceptions
config/kubernetes.yaml                    Kubernetes versions; kind node version for e2e
config/budget.yaml                        single-node memory budget (MiB)
config/e2e.yaml                           test-only values
measurements/                             idle memory and CPU, written by CI
tools/                                    the `catalog` command (Go)
docs/                                     template standards, deploying a template
```

Both kinds of file share one envelope:

```yaml
apiVersion: catalog.zeabur.com/v1
kind: Component                   # or Template
metadata:
  name: Traefik Proxy
  description: Why it is recommended, or what the template runs, in one or two sentences.
  license: MIT                    # SPDX
  homepage: https://traefik.io/traefik/
requires: [certificates]          # <capability> or <capability>/<upstream>
```

The path is the ID: `components/ingress/traefik.yaml` is `ingress/traefik`,
`templates/n8n/` is `n8n`.

## Components

| Capability | Upstream | single-node | multi-node |
| --- | --- | --- | --- |
| ingress | Traefik | recommended | recommended |
| certificates | cert-manager | recommended | recommended |
| observability-operator | VictoriaMetrics operator | recommended | recommended |
| metrics | VictoriaMetrics (k8s stack) | recommended | recommended |
| metrics | metrics-server | recommended | recommended |
| logs | VictoriaLogs (via the operator) | recommended | recommended |
| node-updates | system-upgrade-controller (k3s only) | recommended | recommended |
| intrusion-prevention | CrowdSec | optional | optional |
| volume-backup | K8up | optional | optional |
| volume-backup | Velero (administrators only) | optional | optional |
| event-autoscaling | KEDA | optional | recommended |
| replicated-storage | Longhorn (default storage class on multi-node) | optional | recommended |
| event-autoscaling | KEDA HTTP add-on (beta) | optional | optional |
| uptime-probes | Prometheus blackbox exporter | optional | recommended |
| runtime-security | Falco | optional | optional |
| vulnerability-scanning | Trivy Operator | optional | optional |
| resource-recommendations | Vertical Pod Autoscaler (recommender) | optional | optional |
| remote-access | Tailscale Kubernetes operator | optional | optional |
| config-reload | Stakater Reloader | optional | optional |
| external-secrets | External Secrets Operator (no SecretStore by default) | optional | optional |
| databases | CloudNativePG with the Barman Cloud plugin (PostgreSQL) | optional | optional |
| databases | Percona Operator for MongoDB (server SSPL-1.0, opt-in only) | optional | optional |
| databases | MariaDB Operator (MySQL-compatible) | optional | optional |
| databases | Dragonfly Operator (Redis-compatible; server BUSL-1.1, opt-in only) | optional | optional |
| databases | MOCO (MySQL 8) | optional | optional |
| object-storage | SeaweedFS operator (beta) | optional | optional |
| container-registry | zot | optional | optional |
| image-updates | Keel | optional | optional |
| gotify-notifications | Alertmanager Gotify bridge | optional | optional |

`recommended` is on by default, `optional` is offered but off, `unsupported`
is not offered. Each file's `detection` rules find an existing installation so
it can be reused instead.

## Templates

| ID | Application | Categories |
| --- | --- | --- |
| browserless | Browserless | automation |
| cliproxyapi | CLIProxyAPI | ai |
| dify | Dify | ai |
| error-pages | Error pages | web-servers |
| ghost | Ghost | cms |
| gitea | Gitea | developer-tools |
| halo | Halo | cms |
| hermes-agent-custom-provider | Hermes Agent | ai, automation |
| hermes-agent-openrouter | Hermes Agent | ai, automation |
| librechat | LibreChat | ai |
| litellm | LiteLLM | ai |
| lobechat | LobeChat | ai |
| meilisearch | Meilisearch | search |
| memos | Memos | productivity |
| miniflux | Miniflux | productivity |
| n8n | n8n PostgreSQL | automation |
| n8n-ai-brain | n8n (AI Brain) | ai, automation |
| n8n-sqlite | n8n SQLite | automation |
| new-api | New API | ai |
| nginx | nginx | web-servers |
| ntfy | ntfy | developer-tools |
| ollama | Ollama | ai |
| open-webui | Open WebUI | ai |
| openclaw | OpenClaw | ai |
| openclaw-browser-sandbox | OpenClaw Browser Sandbox | ai, automation |
| openlist | OpenList | storage |
| postgresql-pgvector | PostgreSQL with pgvector | databases |
| qdrant | Qdrant | databases, ai |
| searxng | SearXNG | search |
| sillytavern | SillyTavern | ai |
| sillytavern-legacy | SillyTavern | ai |
| umami | Umami | analytics |
| uptime-kuma | Uptime Kuma | monitoring |
| vaultwarden | Vaultwarden | security |
| wordpress | WordPress | cms |

Categories: `ai`, `analytics`, `automation`, `cms`, `databases`,
`developer-tools`, `monitoring`, `productivity`, `search`, `security`,
`storage`, `web-servers`. [Deploy a template](docs/deploy-a-template.md)
explains how to install one by hand.

## Distributions

Components and templates are tested on kind: one node for single-node, three
for multi-node. They behave the same on other distributions apart from these
known differences:

- k3s: run it with `--disable=traefik,metrics-server` so both come from their
  charts. It installs its own Gateway API CRDs. Behind NAT, leave
  `node-external-ip` unset, or Traefik loses client addresses.
- KubeSolo: its default storage class retains volumes (and their data) after
  their claims are deleted. On Ubuntu, map the hostname to the node IP in
  `/etc/hosts`; with `127.0.1.1` the API server refuses to proxy to the
  kubelet. Its node has no control-plane label.
- Longhorn needs open-iscsi with `iscsid` running, the `iscsi_tcp` module and
  an NFSv4 client on every node, and multipathd kept off its disks; see the
  comments in its file. CI installs them into kind's nodes.
- `distributions` limits a component to some distributions
  (system-upgrade-controller: k3s).
- Templates store data in a StorageClass named `local-path`, which keeps data
  on one node. k3s has it; elsewhere, create it or edit the rendered
  `storageClassName`.

## Component format

Abridged; `schema/v1/component.schema.json` documents every field.

```yaml
apiVersion: catalog.zeabur.com/v1
kind: Component
metadata:
  name: Traefik Proxy
  description: Why it is recommended, in one or two sentences.
  license: MIT                    # SPDX; OSI-approved, or opt-in only (see below)
  homepage: https://traefik.io/traefik/
maturity: independent             # cncf-graduated | cncf-incubating | cncf-sandbox | kubernetes-sig | independent
defaults: {single-node: recommended, multi-node: recommended}
requires: [certificates]          # installed first
distributions: [k3s]              # optional; omitted means any cluster

releases:                         # installed in order
  - name: traefik
    namespace: traefik
    chart: oci://ghcr.io/traefik/helm/traefik   # or repo: https://… plus chart: <name>
    version: 41.7.1                             # exact; Renovate bumps it
    values: |                                   # shared by every profile
      gatewayClass:
        # Every value has a comment saying why; `catalog validate` enforces it.
        enabled: true
    profiles:                                   # merged over values like helm -f
      single-node: |
        deployment:
          # One node: a second replica gains nothing.
          replicas: 1
  - name: crds
    namespace: kube-system
    urls: [https://github.com/…/releases/download/{version}/standard-install.yaml]
    version: v1.6.2
  # or: manifests: |  objects applied as written

inputs: []        # values to ask for or generate; Secrets are created from them
contributes: []   # values added to another component's release (CrowdSec -> Traefik)
detection: {...}  # CRDs, API services, Helm charts, workloads
security: {...}   # RBAC summary, cluster-wide, privileged, egress, credentials, tenant risks
smoke: []         # extra e2e checks
leftovers: []     # what the upstream keeps on uninstall (e.g. CRDs)
```

Installs are deterministic: charts never generate secrets or certificates.
Generated values are `inputs` (stored in Secrets), certificates come from
cert-manager, and `catalog render` fails if two renders differ.

## Template format

Abridged; `schema/v1/template.schema.json` documents every field.

```yaml
apiVersion: catalog.zeabur.com/v1
kind: Template
metadata:
  name: PostgreSQL
  description: PostgreSQL 18 managed by CloudNativePG, with Barman Cloud WAL archiving and scheduled base backups.
  license: PostgreSQL AND Apache-2.0
  homepage: https://www.postgresql.org/
  icon: https://cdn.zeabur.com/marketplace/postgresql.svg
  categories: [databases]

requires: [databases/cloudnative-pg]

services:                         # the workloads in manifests.yaml
  - name: postgresql
    securityProfile: restricted   # or baseline, with baselineJustification
    ports: [{name: postgresql, port: 5432}]
    resources: {requests: {cpu: 100m, memory: 256Mi}, limits: {cpu: "1", memory: 1Gi}}

inputs:
  - name: backupDestination       # the person installing gives it
    description: Dedicated S3 bucket and unique prefix for this cluster.
    required: true
    validation: {pattern: '^s3://[a-z0-9][a-z0-9.-]+/[A-Za-z0-9/_-]+$'}
  # From n8n: the installer generates this value into a Secret.
  # - name: encryptionKey
  #   generate: random
  #   secret: {name: n8n-secrets, key: encryption-key}

security: {...}      # summary, egress destinations, risks with mitigations
scalability: {...}   # replica range and what blocks more
upgradeNotes: []     # what an upgrade needs
smoke: []            # resource condition, then an HTTP or SQL check
```

`manifests.yaml` is a Go template. It gets `.Namespace`, `.Domain`,
`.ClusterIssuer` and `.Inputs.<name>`, and `{{ dnsEgress }}` inserts the
NetworkPolicy rule to cluster DNS. Rendered objects reference Secrets and
never contain generated values.

## Adding a component

1. Pick a capability and an upstream with an OSI-approved license; prefer CNCF
   or Kubernetes SIG projects. A license under `optInOnly` (SSPL-1.0) is
   allowed only with `optional` or `unsupported` in every profile.
2. Create `components/<capability>/<upstream>.yaml` from a similar file. Give
   both profiles values, each with a comment saying why.
3. Check locally (`nix develop` provides Go, Helm, kubectl, kubeconform and
   kind; the e2e tiers also need Docker on Linux):

   ```sh
   go run ./tools/cmd/catalog validate          # schema, license, references, comments
   go run ./tools/cmd/catalog render            # helm template + kubeconform
   go run ./tools/cmd/catalog e2e -tier l0      # what every pull request runs
   go run ./tools/cmd/catalog e2e -tier l2 -profile single-node <id>
   ```

4. Open a pull request with signed-off commits (`git commit -s`).

## Adding a template

1. Create `templates/<id>/template.yaml` and `manifests.yaml` from a similar
   template. The ID is lowercase words joined by hyphens, at most 49
   characters.
2. Meet the [template standards](docs/template-standards.md): pinned images,
   Restricted Pod Security, default-deny NetworkPolicy, resource limits,
   backups for state, and honest scaling limits.
3. Add smoke checks that prove the application answers: a resource condition,
   and an HTTP or SQL check.
4. Check locally. `values.json` is a JSON object with a value for each input
   the template asks for:

   ```sh
   go run ./tools/cmd/catalog validate
   go run ./tools/cmd/catalog template lint -values values.json \
     -domain app.example.test -cluster-issuer letsencrypt <id>
   go run ./tools/cmd/catalog e2e -tier l2 -profile single-node <id>
   ```

5. Open a pull request with signed-off commits (`git commit -s`).

## CI

Tests run in three tiers. A check that one tier leaves out runs in a later
tier. `catalog e2e -tier l0|l1|l2` runs a tier on a Linux machine with Docker,
as CI does.

**L0, every pull request** (`ci.yml`, one job):

1. runs the Go tests;
2. validates every file against its schema, and checks that every template's
   `requires` resolves on both profiles;
3. rejects component licenses that are not OSI-approved, except those under
   `optInOnly` in `config/licenses.yaml` in components no profile recommends;
4. renders every profile twice (output must be identical) and validates it
   with kubeconform for each version in `config/kubernetes.yaml`, checking
   custom resources against the CRDs rendered with them;
5. checks every template, rendered with its test inputs, against the safety
   rules and kubeconform;
6. starts kind with only the rendered CRDs, then sends every rendered
   component (both profiles) and template to the API server with
   `kubectl apply --dry-run=server`. The API server checks fields, defaults,
   CEL rules and Pod Security, and nothing runs.

**L1, in the merge queue and after a merge to main** (`ci.yml`): installs each
changed component and template, and each one that installs a changed
component, on single-node amd64 kind, then runs its smoke checks. Tests that
share dependencies share a cluster, and the dependencies install once per
cluster. A component is uninstalled after its smoke checks unless a later test
in the cluster needs it. A change to `config/`, `tools/`, the flake or a
workflow tests everything.

**L2, every night and before each release** (`e2e.yml`, called by
`nightly.yml` and `release.yml`): tests every component in a fresh kind
cluster for each profile (one node for single-node, three for multi-node), on
amd64 and arm64:

1. installs its dependencies, then the latest release's version when it
   differs, then upgrades to the new version;
2. measures idle memory and CPU every 10 seconds until five samples agree, for
   at most 3 minutes;
3. runs the smoke checks, uninstalls, and fails on anything left behind.

Components listed under `unready` in `config/e2e.yaml` need credentials CI
lacks: they are installed without waiting, smoke tested and uninstalled, not
measured. L2 installs every template on single-node amd64 with the components
it requires, runs its smoke checks, then measures the idle memory and CPU of
the pods in its namespace the same way (`measurements/templates/<id>.json`).
It checks that the single-node
recommended set, measured on amd64, fits `config/budget.yaml`.
`nightly.yml` opens a pull request with fresh `measurements/`; run it by hand
to measure now. Renovate opens a pull request for every new upstream version
and image digest.

Each tier creates its own kind clusters with their own kubeconfig and deletes
them (`-keep` keeps them), and prints how long each phase took. With
`ZEABUR_REGISTRY_MIRRORS` (`docker.io=http://mirror:5001,ghcr.io=...`), the
nodes pull through those mirrors. With `CATALOG_E2E_IMAGES` set to a
directory, L1 keeps one image tarball per image list there and loads it into
the nodes; CI caches the directory.

### Test inputs for templates

Template tests take their input values from `config/e2e.yaml`. They need no
repository secrets, so pull requests run them in full:

- `templateInputs`: values for every template that declares the input.
- `templates.<id>`: values for one template, objects to apply in
  `templates-e2e-<id>` first (such as a Secret that an input names) and smoke
  checks to skip with the reason.

Every value is disposable test data. A value that uses `.S3` starts SeaweedFS
(`object-storage/seaweedfs-operator`) in `templates-e2e-s3`, with HTTPS on TCP
443 from a test CA. Backups and object storage go to it. The Secret `e2e-s3`
in the template's namespace holds its credentials and the CA as `ca.crt`.
Templates take the CA through an optional input such as `backupCASecret`. A
template that cannot use a custom CA skips only that smoke check. The test sets
`kubeApiCidr` from the kind cluster. To test against other storage, pass
`-values` and `-setup` to `catalog template e2e`.

## Releases

Tagging `vX.Y.Z` publishes:

- `index.json`: every component with its install order per profile and
  measurements, recommended components per capability, every template with
  its measurements, the budget and the Kubernetes versions;
- `bundle.tar.gz`: `index.json` plus `components/`, `templates/`, schemas,
  config and measurements;
- `checksums.txt`, and a cosign keyless Sigstore bundle for each.

```sh
cosign verify-blob index.json --bundle index.json.sigstore.json \
  --certificate-identity-regexp '^https://github.com/zeabur-oss/catalog/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Consumers pin a release, verify it, and read `index.json`. `indexVersion`
changes only on incompatible changes. A typical consumer runs `detection`,
offers the recommended components for the cluster's profile, asks for their
`inputs`, and generates installs in `installOrder`, merging `contributes` into
their targets. For a template, it checks that the cluster has everything in
`requires`, asks for the template's inputs, generates the others into Secrets,
and applies `manifests.yaml` rendered for the project's namespace.

## License

The tools and original files are licensed under Apache-2.0. Each upstream,
application, image and chart keeps its own license, named in
`metadata.license`. Some use source-available licenses such as BUSL-1.1 or
SSPL; those components are opt-in only, and those templates ask the person
installing to accept the license before they render. Contributions use the
[Developer Certificate of Origin](https://developercertificate.org/); see
[CONTRIBUTING.md](CONTRIBUTING.md).
