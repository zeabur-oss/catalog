---
default: patch
---

# `databases/mariadb-operator` turns off client-go's streaming list

On a new KubeSolo cluster its controllers started too late to sync their caches, so a MariaDB object was never reconciled.
