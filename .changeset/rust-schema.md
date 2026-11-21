---
default: minor
---

# The schemas come from the Rust types, with TypeScript types

`schema/v1/` is now generated from the types of `crates/zeabur-catalog-format` by `cargo run -p zeabur-catalog -- schema`, which also writes `types/v1/catalog.ts`, the TypeScript types of the same files. The Go `catalog schema` command is gone.

The schemas accept the same files. Their `$defs` have new names, such as `Generate` for `generate`, so a `$ref` into another schema's `$defs` needs the new name. An internally tagged object, such as a release, now lists its fields in each variant of its `oneOf`.
