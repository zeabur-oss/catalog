---
default: patch
---

# Release downloads retry transient failures

`catalog check` and `catalog cluster` now retry a release download that fails with a connection error, a 5xx or a 429 response, up to four attempts with backoff. A 4xx response and a checksum mismatch still fail at once.
