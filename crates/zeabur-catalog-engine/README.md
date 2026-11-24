# zeabur-catalog-engine

What an install of Zeabur Catalog does, decided from the files that `zeabur-catalog-format`
reads. Zeabur and the catalog's own tests call the same code.

- `template`: the objects of a template for one namespace. It follows
  [the rules for clients](https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-clients.mdx),
  and the tests run the cases in `conformance/template/`.
- `digest`: the `sourceDigest` of a component or template directory, Go's `dirhash` `h1:`.
- `kube_version`: whether a component's `kubeVersion` range admits a Kubernetes version.
- `pod_security`: the Pod Security Standards checks of Pod Security Admission, with its messages.
- `quantity`: the canonical text of a resource quantity, as Kubernetes writes it.
