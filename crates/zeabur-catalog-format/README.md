# zeabur-catalog-format

The typed model of Zeabur Catalog and the rules every installer follows.

- `component` and `template`: `components/<id>/component.yaml` and
  `templates/<id>/template.yaml`, read strictly. An unknown field is an error.
- `api`: the files of the static API, `catalog.json`, `components/<id>.json` and
  `templates/<id>.json`, and the measurement files.
- `order`, `select`, `contributions` and `values`: install order, choosing components,
  what components add to each other's releases, and the values of a release.
  [Installers](https://github.com/zeabur-oss/catalog/blob/main/docs/reference/installers.mdx) gives the rules, and
  `conformance/select/` holds the cases the tests run.

`Catalog::read` reads a catalog checkout.
