# zeabur-catalog-engine

What an install of Zeabur Catalog does, decided from the files that `zeabur-catalog-format`
reads. Zeabur and the catalog's own tests call the same code.

- `template`: the objects of a template for one namespace. It follows
  [the rules for clients](https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-clients.mdx),
  and the tests run the cases in `conformance/template/`.
- `digest`: the `sourceDigest` of a component or template directory, Go's `dirhash` `h1:`.
- `kube_version`: whether a component's `kubeVersion` range admits a Kubernetes version.
- `namespace`: the namespaces Kubernetes keeps for itself.
- `objects`: the labels, annotations and field manager an install writes, the K3s/RKE2
  `HelmChart` and `HelmChartConfig`, and the resources discovery serves.
- `pod_security`: the Pod Security Standards checks of Pod Security Admission, with its messages,
  and the levels as namespace labels name them.
- `progress`: how far a chart, an object or a health check has come.
- `quantity`: the canonical text of a resource quantity, as Kubernetes writes it.
- `readiness`: whether an object has reached the state its spec asks for, as kstatus computes it.
