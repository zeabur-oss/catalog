# zeabur-catalog-engine

What an install of Zeabur Catalog does, decided from the files that `zeabur-catalog-format`
reads. Zeabur and the catalog's own tests call the same code.

- `template`: the objects of a template for one namespace. It follows
  [the rules for clients](https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-clients.mdx),
  and the tests run the cases in `conformance/template/`.
