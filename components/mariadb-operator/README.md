## Introduction

MariaDB Operator runs MariaDB from a `MariaDB` object, standalone or replicated. Users, grants and databases are objects too, and backups go to S3. Install it when projects need a MySQL-compatible database under an open-source licence. The operator is MIT. The MariaDB Server images it runs are GPL-2.0.

The component installs two releases in the `mariadb-operator` namespace. One holds the CRDs, so they upgrade before the operator. The other holds the operator and its webhook. It needs cert-manager for the webhook certificate. The install dialog adds cert-manager first when the cluster has none. The operator manages Pods, Services, Secrets, PVCs, Jobs and RBAC for `MariaDB` objects in every namespace.

## Use it

In Zeabur, open a project's **Add** menu and choose **MariaDB**. Set the version and storage, then connect services and set up S3 backups in the database's **Backups** tab. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/).

## Remove it

Removing the component deletes the operator, its webhook and the CRDs. Deleting the CRDs deletes every `MariaDB`, backup, user and grant object. Zeabur refuses while such objects exist, so delete the databases you no longer need first, and back up the rest. The PVCs stay. Delete them by hand when you no longer need the data. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A database never becomes Ready

Check the object and its events with `kubectl -n <namespace> describe mariadb <name>`, and list its Pods with `kubectl -n <namespace> get pods`. Read the operator's logs with `kubectl -n mariadb-operator logs deploy/mariadb-operator`.

### The API server rejects a `MariaDB` object with a webhook error

Check that the webhook Pod runs with `kubectl -n mariadb-operator get pods`, and that its certificate is ready with `kubectl -n mariadb-operator get certificate mariadb-operator-webhook-cert`.
