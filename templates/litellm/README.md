## Introduction

[LiteLLM](https://github.com/BerriAI/litellm) is a proxy that gives you one OpenAI-compatible API in front of many LLM providers. It also has an admin page where you manage models, keys and spend.

This template runs LiteLLM behind your domain, with models, keys and spend stored in PostgreSQL.

## Getting started

1. In Zeabur, open the `litellm` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `litellm.example.com`. The admin page is at `/ui` on that domain.
2. Get the master key. Open **Variables**, click `litellm-secrets` under **From config files and secrets**, and click the eye icon next to `master-key`. Without Zeabur, read Secret `litellm-secrets`, key `master-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret litellm-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

   The key you use is `sk-` followed by this value.
3. Sign in as `admin`. The password is `sk-` followed by the value you just read.
4. The same `sk-` key is the admin API key for requests to `https://litellm.example.com`. Add models and client keys in the admin page.

The key `salt-key` in `litellm-secrets` encrypts the provider credentials in the database. Never change it after you add a model.

## Configuration

### Add a model

In the admin page, open Models + Endpoints and add the model with your provider and its API key. Models you add in the page are stored in the database, so you do not need to edit files or restart. See [Model Management](https://docs.litellm.ai/docs/proxy/model_management).

### Create a key for a client

Do not give the master key to applications. Create a virtual key for each client in the admin page, then point the client at `https://litellm.example.com/v1` and use that key. See [Virtual Keys](https://docs.litellm.ai/docs/proxy/virtual_keys) and [Langchain, OpenAI SDK and other clients](https://docs.litellm.ai/docs/proxy/user_keys).

### Back up the data

LiteLLM keeps models, keys and spend in the `litellm-db` database. It keeps nothing on a volume. In the project, open `litellm-db`. Under **Backups**, set up S3 backups. To get a file instead, click **More** → **Export**. Keep the Secret `litellm-secrets` with the backup, because `salt-key` decrypts the provider credentials in the database. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump) in the Zeabur docs.

## Troubleshooting

### The page shows 502 or does not load

Open the `litellm` service in Zeabur. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

Without Zeabur, read the logs with `kubectl -n <namespace> logs deployment/litellm` and the events with `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur docs.

### Requests fail with a provider error

Read the **Logs** tab of the `litellm` service first. Then, in the admin page, open Models + Endpoints and check the model that fails. The model name must be the one your provider expects. The API key and the endpoint of that model must be valid. And your account must not be over its rate limit or quota at the provider. See [AI Providers](https://docs.litellm.ai/docs/providers) in the LiteLLM docs.
