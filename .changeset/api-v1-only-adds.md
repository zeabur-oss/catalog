---
default: patch
---

# The API on `/v1/` only adds

Every release on `https://oss-catalog.zeabur.com/v1/` keeps each field and type of the last one and makes no field required, whatever its Zeabur Catalog version. A breaking change goes to `/v2/`. CI compares the schemas with those of the latest published release and fails on such a change.
