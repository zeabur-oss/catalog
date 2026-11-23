---
default: minor
---

# The schemas come from the Rust types, with TypeScript types

`schema/v1/` is now generated from the types of `crates/zeabur-catalog-format` by `cargo run -p zeabur-catalog -- schema`, which also writes `types/v1/catalog.ts`, the TypeScript types of the same files. The Go `catalog schema` command is gone.

Every file of the catalog passes them, as it passed the schemas before. Their `$defs` have new names, such as `Generate` for `generate`, so a `$ref` into another schema's `$defs` needs the new name. An internally tagged object, such as a release, now lists its fields in each variant of its `oneOf`.

A quantity in `resources.work` is a string, such as `cpu: "1"` or `memory: 256Mi`, as the Rust reader reads it. A number there no longer passes the schema. This changes a field's type inside v1, which is why this release is a new baseline: every published file already writes its quantities as strings.

CI no longer compares the text of the schemas with the latest release's. It checks that every file of the new API passes the latest release's schemas, with new fields and values allowed, and its own, because readers depend on the data. A schema change that no file of the catalog uses is not seen by the check. These schemas are the new baseline.
