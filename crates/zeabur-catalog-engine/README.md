# zeabur-catalog-engine

What an install of Zeabur Catalog does, decided from the files that `zeabur-catalog-format`
reads. Zeabur and the catalog's own tests call the same code.

- `template`: the objects of a template for one namespace. It follows
  [the rules for clients](https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-clients.mdx),
  and the tests run the cases in `conformance/template/`.
- `digest`: the `sourceDigest` of a component or template directory, Go's `dirhash` `h1:`.
- `kube_version`: whether a component's `kubeVersion` range admits a Kubernetes version.
- `namespace`: the namespaces Kubernetes keeps for itself.
- `pod_security`: the Pod Security Standards checks of Pod Security Admission, with its messages,
  and the levels as namespace labels name them.
- `quantity`: the canonical text of a resource quantity, as Kubernetes writes it.
- `readiness`: whether an object has reached the state its spec asks for, as kstatus computes it.
