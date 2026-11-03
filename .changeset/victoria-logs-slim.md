---
default: patch
---

# VictoriaLogs uses less memory

The log store limits its caches to 128 MiB and runs two searches at a time, and the collector keeps at most 512 MiB of lines for a log store that is down. VictoriaLogs still keeps every namespace's logs. On a single node it works with 64 MiB over its idle use instead of 350 MiB, and its memory limits fall to 384 MiB and 128 MiB.
