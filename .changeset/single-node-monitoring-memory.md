---
default: patch
---

# A personal install leaves VictoriaMetrics and VictoriaLogs off machines with less than about 1.7 GiB of memory

`victoria-metrics` and `victoria-logs` give `resources.workMemoryMiB`, measured on KubeSolo under load. The VictoriaMetrics operator is now optional on `single-node`, so it is installed only with them.
