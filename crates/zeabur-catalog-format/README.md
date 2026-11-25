# zeabur-catalog-format

The typed model of Zeabur Catalog and the rules every installer follows.

- `component` and `template`: `components/<id>/component.yaml` and
  `templates/<id>/template.yaml`.
- `api`: the files of the static API, `catalog.json`, `components/<id>.json` and
  `templates/<id>.json`, and the measurement files.
- `order`, `select`, `contributions` and `values`: install order, choosing components,
  what components add to each other's releases, and the values of a release.
  [Installers](https://github.com/zeabur-oss/catalog/blob/main/docs/reference/installers.mdx) gives the rules, and
  `conformance/select/` holds the cases the tests run.

Readers of the API decode the types directly: they ignore a field they do not know and read an
unknown value as `Unknown`, as [releases](https://github.com/zeabur-oss/catalog/blob/main/docs/reference/releases.mdx)
says. `check` reads the files authors write strictly and names each unknown field or value by its
path. `Catalog::read` reads a catalog checkout through it.

The `utoipa` feature, off by default, derives `utoipa::ToSchema` on the types a reader puts in its
own OpenAPI document, so the reader needs no copy of them. It covers `InputValue`.

The `schema` feature, off by default, derives `schemars::JsonSchema` and `ts_rs::TS` and adds the
`schema` module, which writes `schema/v1/` and `types/v1/`. Only this repository needs it.
