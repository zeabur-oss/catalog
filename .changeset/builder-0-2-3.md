---
default: patch
---

# Zeabur Builder 0.2.3

The `builder` component installs chart 0.2.3. Builds can be capped per cluster, run on build nodes set aside by a taint, and record their steps and kstatus conditions. Build Jobs get a memory limit, and the images are signed by Builder's own release workflow.
