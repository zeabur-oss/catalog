---
default: patch
---

# Longhorn's node check names no installer

When a node isn't ready for Longhorn, the advice says to install what the node lacks, such as open-iscsi, an NFS client or a kernel module. It no longer tells you to run zeabur-installer, which Zeabur removed.
