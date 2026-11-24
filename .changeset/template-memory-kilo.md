---
default: patch
---

# A template memory writes kilo as lowercase `k`

`resources.memory` in a template takes the Kubernetes suffixes: `64k` passes and `64K`, which the API server refuses, fails `zeabur-catalog check`. `Pi`, `Ei`, `P` and `E` pass too.
