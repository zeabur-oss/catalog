---
default: minor
---

# Ghost runs its MySQL on MOCO

A new Ghost deploy creates a MOCO `MySQLCluster`, `ghost-mysql`, in place of the plain `ghost-mysql` StatefulSet, and requires the `moco` component. Zeabur OSS shows it as a MySQL database with a **Backups** tab. A job makes the `ghost` database and user, and the user's password is in the Secret `ghost-mysql-app`. The `mysqlRootPassword` input is gone. A Ghost you already deployed keeps its StatefulSet.

`catalog template lint` checks MOCO `MySQLCluster` objects: the template's `moco` requirement, a pinned `mysqld` image with requests and limits, the template label on its pods, and restricted security contexts on `mysqld` and on each container MOCO adds.
