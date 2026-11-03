---
default: patch
---

# VictoriaLogs collects only what the history tab reads

The collector skips Kubernetes' and the catalog's own namespaces, drops fields the history tab never shows, and keeps at most 512 MiB of lines for a log store that is down. The log store limits its caches to 128 MiB and runs two searches at a time. On a single node, VictoriaLogs works with 64 MiB over its idle use instead of 350 MiB, and its memory limits fall to 384 MiB and 128 MiB.
