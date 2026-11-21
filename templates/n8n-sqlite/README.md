## Introduction

[n8n](https://github.com/n8n-io/n8n) is a workflow automation tool. You connect apps and services with nodes, and n8n runs the workflows on a trigger or a schedule.

This template runs one n8n behind your domain, with SQLite on a volume. Code nodes run in a separate `task-runner` Pod. The owner account is created at the start, so there is no setup screen.

## Getting started

1. In Zeabur OSS, open the `n8n` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `n8n.example.com`.
2. Sign in with the owner email you entered in the install form. Get the password from the `n8n` service: open **Variables**, click `n8n-secrets` under **From config files and secrets**, and click the eye icon next to `owner-password`. Without Zeabur OSS, read Secret `n8n-secrets`, key `owner-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. Keep a copy of the encryption key in a safe place. n8n encrypts the credentials it saves with this key. Find it the same way: on **Variables**, click `n8n-secrets`, then the eye icon next to `encryption-key`. Without Zeabur OSS, read Secret `n8n-secrets`, key `encryption-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. Secret `n8n-secrets` also holds `runner-token`, the shared secret that n8n and the task runner use to trust each other. You do not need it for daily use. Show it the same way on **Variables**. Without Zeabur OSS, use kubectl:

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   ```

## Configuration

### Change the owner password

n8n sets the owner account from the install form and the Secret at every start, and shows the owner details as read-only in its interface. To change the password, open **Variables** on the `n8n` service, click `n8n-secrets` under **From config files and secrets**, and enter a new value for `owner-password`. Review and apply, then click **Restart** at the top of the service page.

See [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email) in the n8n docs.

### Call a workflow with a webhook

The Webhook node has two URLs. The test URL works while you select Listen for Test Event, and shows the incoming data in the editor. The production URL works after you publish the workflow. Both start with your domain. See [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/) in the n8n docs.

### Back up or move the data

n8n keeps its SQLite database and its files on the `n8n-data` volume, mounted at `/home/node/.n8n`. n8n advises you to stop it before you copy the SQLite file, so export your workflows and credentials with the n8n command line instead.

1. Keep the encryption key from Getting started. Without it, n8n cannot read the saved credentials.
2. Export the workflows and credentials. Click **Terminal** at the top of the `n8n` service page and run:

   ```sh
   n8n export:workflow --backup --output=/home/node/.n8n/backups/workflows/
   n8n export:credentials --backup --output=/home/node/.n8n/backups/credentials/
   ```

   Then open the **Files** tab of the `n8n` service and download these two folders. Do not download the whole volume. A big folder can time out.

3. Copy the files that workflows saved. This template stores them on the volume, in `/home/node/.n8n/binaryData`, and neither the database nor the export holds them. The folder can be big, so stream it to your machine with kubectl instead of the **Files** tab:

   ```sh
   kubectl -n <namespace> exec deploy/n8n -c n8n -- busybox tar czf - -C /home/node/.n8n binaryData > n8n-binary-data.tgz
   ```

   To put it back on a new `n8n`, run `kubectl -n <namespace> exec -i deploy/n8n -c n8n -- busybox tar xzf - -C /home/node/.n8n < n8n-binary-data.tgz`. See [Binary data environment variables](https://docs.n8n.io/hosting/configuration/environment-variables/binary-data/) in the n8n docs.

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

Code nodes run in the `task-runner` service, which connects to n8n with `runner-token`. Open the `task-runner` service in Zeabur OSS, check that **Overview** shows its replica as ready, and read the **Logs** tab.

See [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners) in the n8n docs.
