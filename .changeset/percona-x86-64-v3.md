---
default: patch
---

# Percona MongoDB and LibreChat say that the operator image needs an x86-64-v3 CPU

The `percona-mongodb` summary and READMEs, and the LibreChat risks, say that the operator image 1.22.0 and later is built on UBI 10 and needs an x86-64-v3 CPU on x86 nodes, with a command that checks a node.
