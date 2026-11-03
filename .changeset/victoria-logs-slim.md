---
default: patch
---

# VictoriaLogs uses less memory and keeps the audit log

The log store limits its caches to 128 MiB and runs two searches at a time, and the collector keeps at most 512 MiB of lines for a log store that is down. On a single node VictoriaLogs works with 64 MiB over its idle use instead of 350 MiB, and its memory limits fall to 384 MiB and 128 MiB. The collector leaves out the logs of kube-system and observability, which only add noise to the 8 GiB the logs share, and now also keeps kube-apiserver's audit log as its own stream, so you can search who did what.
