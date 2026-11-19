English | [繁體中文](README.zh-TW.md) | [简体中文](README.zh-CN.md)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/cover-dark.avif">
  <img alt="Zeabur Catalog: every template is installed and tested first. It just runs. Logos of WordPress, n8n, Ghost, Uptime Kuma, Vaultwarden, Umami, Gitea, Ollama, SearXNG and ntfy." src="docs/images/cover-light.avif">
</picture>

# Zeabur Catalog

One-click templates for your own server: n8n, WordPress, Dify, OpenClaw and more, each installed
and tested before it ships. It is the open-source counterpart of zeabur.com's template
marketplace.

## Using Zeabur?

The catalog is already built into Zeabur. In a project, select **Add** → **App template**, pick an
application and deploy it. See
[Deploy from an application template](https://oss.zeabur.com/docs/zeabur/deploy/templates/) in
the Zeabur docs.

Coming from zeabur.com? [Templates from zeabur.com](docs/get-started/zeabur-com-templates.mdx)
lists the catalog template for each popular zeabur.com template. The
[Zeabur docs](https://oss.zeabur.com/docs/zeabur/) cover moving your services to your own server.

If you use Zeabur, you can stop reading here.

## What it does for you

- **Tested before it ships.** Before any change to a template is merged, CI installs it on a
  Kubernetes cluster and checks that the application answers.
- **Fixed versions.** Each template pins its images, so the same template installs the same
  application every time. A new version arrives when you upgrade Zeabur.
- **Passwords generated for you.** Passwords and keys are generated when you deploy and stored as
  Kubernetes Secrets, never written into the template.
- **Known resource needs.** Each template states the CPU and memory it needs, and runs without
  root where the application allows.

## Use it on its own

Without Zeabur, the catalog is tested component and template data for Kubernetes clusters, with
one signed bundle and a static API per release. A component is installed once for the whole
cluster and used by every project; a template is installed per project and can be installed many
times.

- **Components** are what a cluster administrator installs once per cluster:
  ingress, certificates, metrics, database operators. For each capability the
  catalog says which upstream chart, which version, which values, and why.
- **Templates** are applications a project member installs into one namespace:
  n8n, Dify, WordPress, PostgreSQL. Each pins its images, sets
  resource requests and limits, and runs under the Restricted Pod Security
  Standard where the application allows it. A template names the components
  it needs in `requires`.

The catalog is data. CI installs and tests every component and template, and
each release publishes one signed bundle and a static API that any tool can read to generate
`helm install` commands, Flux or Argo CD manifests, k3s `HelmChart` objects or
template installs.

[Zeabur](https://github.com/zeabur-oss/zeabur) reads this catalog to suggest what
a cluster is missing; it never installs components itself.

### Layout

```text
components/<id>.yaml                      one file per upstream; provides names its capability
templates/<id>/template.yaml              one application: inputs, services, security, smoke checks
templates/<id>/manifests.yaml             its Kubernetes objects
templates/<id>/e2e.yaml                       test-only inputs of that template (optional)
schema/v1/                                JSON Schemas for both
config/licenses.yaml                      OSI-approved SPDX identifiers; opt-in-only exceptions
config/kubernetes.yaml                    Kubernetes versions; kind node version for e2e
config/budget.yaml                        single-node memory budget (MiB)
config/e2e.yaml                           test-only values
conformance/                              test cases for installers and template clients
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
requires: [{capability: certificates}]   # or {component: <id>}
```

The path is the ID: `components/traefik.yaml` is `traefik`, `templates/n8n/` is
`n8n`. A component names the capability it offers in `provides`, such as
`provides: {capability: ingress}`. No component ID is also a template ID.

## Documentation

For Zeabur users:

- [Deploy from an application template](https://oss.zeabur.com/docs/zeabur/deploy/templates/) and [view and install cluster components](https://oss.zeabur.com/docs/zeabur/cluster/components/), in the Zeabur docs
- [Templates from zeabur.com](docs/get-started/zeabur-com-templates.mdx)
- [Template list](docs/reference/templates.mdx)

For using the catalog on its own:

- [Deploy a template](docs/get-started/deploy-a-template.mdx)
- [Components](docs/reference/components.mdx), [templates](docs/reference/templates.mdx) and [distributions](docs/reference/distributions.mdx)
- [Component format](docs/reference/component-format.mdx) and [template format](docs/reference/template-format.mdx)
- [Template standards](docs/reference/template-standards.mdx)
- [Releases and the API](docs/reference/releases.mdx)
- [Add a component](docs/contribute/add-component.mdx), [add a template](docs/contribute/add-template.mdx) and [tests and CI](docs/contribute/tests.mdx)

## License

The tools and original files are licensed under Apache-2.0. Each upstream,
application, image and chart keeps its own license, named in
`metadata.license`. Some use source-available licenses such as BUSL-1.1 or
SSPL; those components are opt-in only, and those templates name and link the
license in a comment at the top of `template.yaml`. Contributions use the
[Developer Certificate of Origin](https://developercertificate.org/); see
[CONTRIBUTING.md](CONTRIBUTING.md).
