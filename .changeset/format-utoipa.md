---
default: minor
---

# An optional utoipa feature in zeabur-catalog-format

With the `utoipa` feature, `InputValue` implements `utoipa::ToSchema`, so a reader that builds its OpenAPI document with utoipa uses the type itself instead of a copy. The feature is off by default.
