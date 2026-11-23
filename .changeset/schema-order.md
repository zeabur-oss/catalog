---
default: patch
---

# Each schema starts with its $schema, $id, title and description

Every file of `schema/v1/` lists `$schema`, `$id`, `title` and `description` first, before `$defs`. `template-case.schema.json` has its `$id` under `https://oss-catalog.zeabur.com/v1/schema/`, as the other schemas do.
