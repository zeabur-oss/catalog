## Introduction

[n8n](https://github.com/n8n-io/n8n) is a workflow automation tool. You connect apps and services with nodes, and n8n runs the workflows on a trigger or a schedule.

This template runs one n8n behind your domain, with its own PostgreSQL database. Code nodes run in a separate `n8n-runners` Pod, which also runs Python. The owner account is created at the start, so there is no setup screen.

## Getting started

1. In Zeabur OSS, open the `n8n` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `n8n.example.com`.
2. Sign in with the owner email you entered in the install form. Get the password from the `n8n` service: open **Variables**, click `app-secrets` under **From config files and secrets**, and click the eye icon next to `owner-password`. Without Zeabur OSS, read Secret `app-secrets`, key `owner-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. Keep a copy of the encryption key in a safe place. n8n encrypts the credentials it saves with this key. Find it the same way: on **Variables**, click `app-secrets`, then the eye icon next to `encryption-key`. Without Zeabur OSS, read Secret `app-secrets`, key `encryption-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. Secret `app-secrets` also holds two keys that you do not need for daily use. `runner-token` is the shared secret that n8n and the code runner use to trust each other. `jwt-key` is the secret for user management. Show them the same way on **Variables**. Without Zeabur OSS, use kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.jwt-key}' | base64 -d
   ```

## Configuration

### Change the owner password

n8n sets the owner account from the install form and the Secret at every start, and shows the owner details as read-only in its interface. To change the password, open **Variables** on the `n8n` service, click `app-secrets` under **From config files and secrets**, and enter a new value for `owner-password`. Review and apply, then click **Restart** at the top of the service page.

See [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email) in the n8n docs.

### Call a workflow with a webhook

The Webhook node has two URLs. The test URL works while you select Listen for Test Event, and shows the incoming data in the editor. The production URL works after you publish the workflow. Both start with your domain. See [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/) in the n8n docs.

### Back up or move the data

n8n keeps workflows, credentials and execution history in the `n8n-db` database. The `n8n-data` volume, mounted at `/home/node/.n8n`, holds files such as binary data. A backup needs the database, the encryption key and, to move to another n8n, an export of your workflows and credentials.

1. Back up the database. In the project, open the `n8n-db` database. Under **Backups**, set up S3 backups. To get a file instead, click **More** → **Export**. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump) in the Zeabur OSS docs.
2. Keep the encryption key from Getting started. Without it, n8n cannot read the saved credentials.
3. Export the workflows and credentials. Click **Terminal** at the top of the `n8n` service page and run:

   ```sh
   n8n export:workflow --backup --output=/home/node/.n8n/backups/workflows/
   n8n export:credentials --backup --output=/home/node/.n8n/backups/credentials/
   ```

   Then open the **Files** tab of the `n8n` service and download these two folders. Do not download the whole volume. A big folder can time out.

To move to another n8n, upload the two folders on the **Files** tab of the new `n8n` service and run the import with **Terminal**:

```sh
n8n import:workflow --separate --input=<workflows folder>
n8n import:credentials --separate --input=<credentials folder>
```

The new n8n must use the same encryption key, or it cannot read the credentials. If it cannot, add `--decrypted` to `n8n export:credentials`. That file holds the credentials in plain text, so delete it after the import. The export holds no users, execution history, variables or instance settings.

See [Back up and restore](https://docs.n8n.io/deploy/host-n8n/keep-n8n-running/backup-and-restore) and [Use the command line](https://docs.n8n.io/deploy/host-n8n/configure-n8n/use-the-command-line) in the n8n docs.

## Troubleshooting

### The page shows 502, does not load or keeps reconnecting

Open the `n8n` service in Zeabur OSS. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. A service that is slow or keeps reconnecting is often short of memory too, so check **Events** and **Maximum memory** for it as well. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur OSS docs.

### A Code node does not run

Code nodes run in the `n8n-runners` service, which connects to n8n with `runner-token`. Open the `n8n-runners` service in Zeabur OSS, check that **Overview** shows its replica as ready, and read the **Logs** tab.

See [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners) in the n8n docs.

### Code in a Code node cannot reach the network

The `n8n-runners` Pod can reach only DNS and the n8n task broker, because the template limits its network with a NetworkPolicy. A Code node that calls an outside address fails. Fetch the data with an HTTP Request node, and pass it to the Code node.
