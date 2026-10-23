<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/cover-dark.avif">
  <img alt="Zeabur Catalog: which chart, which version, which values, and why, for each Kubernetes component, installed and measured in CI" src="docs/images/cover-light.avif">
</picture>

# Zeabur Catalog

A tested catalog of Kubernetes components: for each capability, which upstream
chart, which version, which values, and why. It is data, not an installer.
CI installs and measures every component, and each release publishes a signed
bundle any tool can read to generate `helm install` commands, Flux or Argo CD
manifests, or k3s `HelmChart` objects.

[Zeabur](https://github.com/zeabur/zeabur) reads this catalog to suggest what a
cluster is missing; it never installs anything itself.

## Layout

```text
capabilities/<capability>/<upstream>.yaml   one file per capability × upstream
schema/v1/component.schema.json             JSON Schema for those files
config/licenses.yaml                        OSI-approved SPDX identifiers; opt-in-only exceptions
config/kubernetes.yaml                      Kubernetes versions; kind node version for e2e
config/budget.yaml                          single-node memory budget (MiB)
config/e2e.yaml                             test-only values
measurements/                               idle memory and CPU, written by CI
tools/                                      the `catalog` command (Go)
```

## Capabilities

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
| dns | ExternalDNS | optional | optional |
| resource-recommendations | Vertical Pod Autoscaler (recommender) | optional | optional |
| remote-access | Tailscale Kubernetes operator | optional | optional |
| config-reload | Stakater Reloader | optional | optional |
| databases | CloudNativePG with the Barman Cloud plugin (PostgreSQL) | optional | optional |
| databases | Percona Operator for MongoDB (server SSPL-1.0, opt-in only) | optional | optional |
| databases | MariaDB Operator (MySQL-compatible) | optional | optional |
| databases | Dragonfly Operator (Redis-compatible; server BUSL-1.1, opt-in only) | optional | optional |
| databases | MOCO (MySQL 8) | optional | optional |
| object-storage | SeaweedFS operator (beta) | optional | optional |
| container-registry | zot | optional | optional |
| image-updates | Keel | optional | optional |

`recommended` is on by default, `optional` is offered but off, `unsupported`
is not offered. Each file's `detection` rules find an existing installation so
it can be reused instead.

## Distributions

Components are tested on kind: one node for single-node, three for
multi-node. They behave the same on other distributions apart from these
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


## Data format

The path names the component: `capabilities/<capability>/<upstream>.yaml`.
Abridged; the schema documents every field.

```yaml
displayName: Traefik Proxy
homepage: https://traefik.io/traefik/
license: MIT                      # SPDX; OSI-approved, or opt-in only (see below)
maturity: independent             # cncf-graduated | cncf-incubating | cncf-sandbox | kubernetes-sig | independent
rationale: Why it is recommended, in one or two sentences.
defaults: {single-node: recommended, multi-node: recommended}
dependsOn: [certificates]         # <capability> or <capability>/<upstream>
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

## Adding an upstream

1. Pick a capability and an upstream with an OSI-approved license; prefer CNCF
   or Kubernetes SIG projects. A license under `optInOnly` (SSPL-1.0) is
   allowed only with `optional` or `unsupported` in every profile.
2. Create `capabilities/<capability>/<upstream>.yaml` from a similar file. Give
   both profiles values, each with a comment saying why.
3. Check locally (`nix develop` provides Go, Helm, kubectl, kubeconform and
   kind):

   ```sh
   go run ./tools/cmd/catalog validate               # schema, license, references, comments
   go run ./tools/cmd/catalog render                 # helm template + kubeconform
   kind create cluster                            # --config .github/kind.yaml for multi-node
   go run ./tools/cmd/catalog e2e -profile single-node <id>
   ```

4. Open a pull request with signed-off commits (`git commit -s`).

## CI

Every pull request:

1. validates every file against the schema;
2. rejects licenses that are not OSI-approved, except those under `optInOnly`
   in `config/licenses.yaml` in components no profile recommends;
3. renders every profile twice (output must be identical) and validates it
   with kubeconform for each version in `config/kubernetes.yaml`, checking
   custom resources against the CRDs rendered with them;
4. installs each changed component and its dependents (every component when
   `config/`, `tools/` or the cluster changes) on kind for each profile, on
   amd64 and arm64: dependencies first, smoke test, idle memory and CPU,
   uninstall, and fails on anything left behind (components listed under
   `unready` in `config/e2e.yaml` need credentials CI lacks: they are installed
   without waiting, smoke tested and uninstalled, not measured);
5. checks that the single-node recommended set, measured on amd64, fits
   `config/budget.yaml`;
6. upgrades from the latest release's version when it changed.

After merges, `measure.yml` measures the same components and opens a pull
request with fresh `measurements/`; run it by hand to measure everything. Renovate opens a pull request for every new upstream version.

## Releases

Tagging `vX.Y.Z` publishes:

- `index.json`: every component with its install order per profile and
  measurements, recommended components per capability, the budget and the
  Kubernetes versions;
- `bundle.tar.gz`: `index.json` plus sources, schema, config and measurements;
- `checksums.txt`, and a cosign keyless Sigstore bundle for each.

```sh
cosign verify-blob index.json --bundle index.json.sigstore.json \
  --certificate-identity-regexp '^https://github.com/zeabur/catalog/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Consumers pin a release, verify it, and read `index.json`. `indexVersion`
changes only on incompatible changes. A typical consumer runs `detection`,
offers the recommended components for the cluster's profile, asks for their
`inputs`, and generates installs in `installOrder`, merging `contributes` into
their targets.

## License

Apache-2.0. Contributions use the
[Developer Certificate of Origin](https://developercertificate.org/); see
[CONTRIBUTING.md](CONTRIBUTING.md).
