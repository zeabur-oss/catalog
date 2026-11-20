## Introduction

[n8n](https://github.com/n8n-io/n8n) is a workflow automation tool. You connect apps and services with nodes, and n8n runs the workflows on a trigger or a schedule.

This template runs one n8n behind your domain, with SQLite on a volume. Code nodes run in a separate `task-runner` Pod. The owner account is created at the start, so there is no setup screen.

## Getting started

1. In Zeabur, open the `n8n` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `n8n.example.com`.
2. Sign in with the owner email you entered in the install form. Get the password from the `n8n` service: open **Variables**, click `n8n-secrets` under **From config files and secrets**, and click the eye icon next to `owner-password`. Without Zeabur, read Secret `n8n-secrets`, key `owner-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. Keep a copy of the encryption key in a safe place. n8n encrypts the credentials it saves with this key. Find it the same way: on **Variables**, click `n8n-secrets`, then the eye icon next to `encryption-key`. Without Zeabur, read Secret `n8n-secrets`, key `encryption-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. Secret `n8n-secrets` also holds `runner-token`, the shared secret that n8n and the task runner use to trust each other. You do not need it for daily use. Show it the same way on **Variables**. Without Zeabur, use kubectl:

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   ```

## Configuration

### Change the owner password

n8n sets the owner account from the install form and the Secret at every start, and shows the owner details as read-only in its interface. To change the password, open **Variables** on the `n8n` service, click `n8n-secrets` under **From config files and secrets**, and enter a new value for `owner-password`. Review and apply, then click **Restart** at the top of the service page.

See [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email) in the n8n docs.

### Call a workflow with a webhook

The Webhook node has two URLs. The test URL works while you select Listen for Test Event, and shows the incoming data in the editor. The production URL works after you publish the workflow. Both start with your domain. See [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/) in the n8n docs.

## Troubleshooting

### A Code node does not run

Code nodes run in the `task-runner` service, which connects to n8n with `runner-token`. Open the `task-runner` service in Zeabur, check that **Overview** shows its replica as ready, and read the **Logs** tab.

See [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners) in the n8n docs.
