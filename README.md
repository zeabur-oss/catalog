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
  resource requests and limits, and runs under the Restricted Pod Security
  Standard where the application allows it. A template names the components
  it needs in `requires`.

The catalog is data. CI installs and tests every component and template, and
each release publishes one signed bundle that any tool can read to generate
`helm install` commands, Flux or Argo CD manifests, k3s `HelmChart` objects or
template installs.

[Zeabur](https://github.com/zeabur-oss/zeabur) reads this catalog to suggest what
a cluster is missing; it never installs anything itself.

## Layout

```text
components/<capability>/<upstream>.yaml   one file per capability × upstream
templates/<id>/template.yaml              one application: inputs, services, security, smoke checks
templates/<id>/manifests.yaml             its Kubernetes objects
schema/v1/                                JSON Schemas for both
config/licenses.yaml                      OSI-approved SPDX identifiers; opt-in-only exceptions
config/kubernetes.yaml                    Kubernetes versions; kind node version for e2e
config/budget.yaml                        single-node memory budget (MiB)
config/e2e.yaml                           test-only values
measurements/                             idle memory and CPU, written by CI
tools/                                    the `catalog` command (Go)
docs/                                     the documentation
```

Both kinds of file share one envelope:

```yaml
apiVersion: catalog.zeabur.com/v1
kind: Component                   # or Template
metadata:
  name: Traefik Proxy
  license: MIT                    # SPDX
  homepage: https://traefik.io/traefik/
requires: [certificates]          # <capability> or <capability>/<upstream>
```

The path is the ID: `components/ingress/traefik.yaml` is `ingress/traefik`,
`templates/n8n/` is `n8n`.

## Documentation

- [Deploy a template](docs/get-started/deploy-a-template.mdx)
- [Components](docs/reference/components.mdx), [templates](docs/reference/templates.mdx) and [distributions](docs/reference/distributions.mdx)
- [Component format](docs/reference/component-format.mdx) and [template format](docs/reference/template-format.mdx)
- [Template standards](docs/reference/template-standards.mdx)
- [Releases and the bundle](docs/reference/releases.mdx)
- [Add a component](docs/contribute/add-component.mdx), [add a template](docs/contribute/add-template.mdx) and [tests and CI](docs/contribute/tests.mdx)

## License

The tools and original files are licensed under Apache-2.0. Each upstream,
application, image and chart keeps its own license, named in
`metadata.license`. Some use source-available licenses such as BUSL-1.1 or
SSPL; those components are opt-in only, and those templates name and link the
license in a comment at the top of `template.yaml`. Contributions use the
[Developer Certificate of Origin](https://developercertificate.org/); see
[CONTRIBUTING.md](CONTRIBUTING.md).
