---
default: patch
---

# zeabur-catalog-format builds without schemars and ts-rs

`schemars` and `ts-rs` are behind the new `schema` feature, which is off by default. A reader that only parses the files no longer compiles them or their dependencies. `schema/v1/` and `types/v1/` do not change.
