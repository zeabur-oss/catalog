## Introduction

[n8n](https://github.com/n8n-io/n8n) is a workflow automation tool. You connect apps and services with nodes, and n8n runs the workflows on a trigger or a schedule.

This template runs one n8n behind your domain, with its own PostgreSQL database. Code nodes run in a separate `n8n-runners` Pod, which also runs Python. The owner account is created at the start, so there is no setup screen.

## Getting started

1. In Zeabur, open the `n8n` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `n8n.example.com`.
2. Sign in with the owner email you entered in the install form. Get the password from the `n8n` service: open **Variables**, click `app-secrets` under **From config files and secrets**, and click the eye icon next to `owner-password`. Without Zeabur, read Secret `app-secrets`, key `owner-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. Keep a copy of the encryption key in a safe place. n8n encrypts the credentials it saves with this key. Find it the same way: on **Variables**, click `app-secrets`, then the eye icon next to `encryption-key`. Without Zeabur, read Secret `app-secrets`, key `encryption-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. Secret `app-secrets` also holds two keys that you do not need for daily use. `runner-token` is the shared secret that n8n and the code runner use to trust each other. `jwt-key` is the secret for user management. Show them the same way on **Variables**. Without Zeabur, use kubectl:

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

## Troubleshooting

### A Code node does not run

Code nodes run in the `n8n-runners` service, which connects to n8n with `runner-token`. Open the `n8n-runners` service in Zeabur, check that **Overview** shows its replica as ready, and read the **Logs** tab.

See [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners) in the n8n docs.

### Code in a Code node cannot reach the network

The `n8n-runners` Pod can reach only DNS and the n8n task broker, because the template limits its network with a NetworkPolicy. A Code node that calls an outside address fails. Fetch the data with an HTTP Request node, and pass it to the Code node.
