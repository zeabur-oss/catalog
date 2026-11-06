---
default: patch
---

# Operators on `multi-node` request what they use

On `multi-node`, each replica of the CloudNativePG, Dragonfly, MariaDB, MOCO, Percona MongoDB and SeaweedFS operators, Reloader and k8up now requests what it does on `single-node`, half or less of before. On a two-node cluster with apps, each leader uses 20–54 MiB, under its request. Their limits stay as they were, so they still have room as the objects they manage grow. The CloudNativePG Barman Cloud plugin requests 32 MiB on both profiles instead of 24 and 64, because it uses about 30 MiB on a cluster with apps.
