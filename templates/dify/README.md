## Introduction

[Dify](https://github.com/langgenius/dify) is a platform for building LLM apps. You design workflows, agents and chat apps in a web editor, and publish them as web apps or APIs.

This template runs Dify behind your domain, with its API, worker, plugin daemon, code sandbox, PostgreSQL and a queue. Dify keeps documents and uploaded files in the S3-compatible storage you entered in the install form.

## Getting started

1. In Zeabur, open the `dify` service. When **Overview** shows its replicas as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `dify.example.com`. Dify also needs its other services, such as `dify-api`, to be ready.
2. Get the initialization password. Open the `dify-api` service, open **Variables**, click `dify-secrets` under **From config files and secrets**, and click the eye icon next to `init-password`. Without Zeabur, read Secret `dify-secrets`, key `init-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret dify-secrets -o jsonpath='{.data.init-password}' | base64 -d
   ```

3. Open `https://dify.example.com/install`. Dify asks for the initialization password first, so nobody else can claim the instance. Enter it, then create the administrator account.
4. Sign in at `https://dify.example.com` with the account you created.

The other keys in `dify-secrets` (`app-key`, `redis-password` and `sandbox-key`) are for Dify's own services. Do not share them.

## Configuration

### Add a model provider

Dify has no models until you connect a provider. In Dify, open Integrations > Model Provider, install the provider you use, click Add API Key, and enter your key. Dify checks the key before it makes the provider available. See [Model Providers](https://docs.dify.ai/en/use-dify/workspace/model-providers).

### Back up the data

Dify keeps accounts, apps and settings in the `dify-db` database, which holds the two databases `dify` and `dify_plugin`. In the project, open `dify-db`. Under **Backups**, set up S3 backups. A backup of `dify-db` covers both databases. Documents and uploaded files are in the S3 bucket you entered in the install form, so back up that bucket with your storage service. Installed plugins are on the volume `dify-plugin-storage`, mounted at `/app/storage`.

Keep the Secret `dify-secrets` with the backup. Dify uses `app-key` to sign tokens and file URLs and to encrypt OAuth client credentials. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) in the Zeabur docs and [SECRET_KEY](https://docs.dify.ai/en/self-host/configuration/environments) in the Dify docs.

## Troubleshooting

### The page shows 502 or does not load

Open the `dify` service in Zeabur. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the service is slow or keeps reconnecting, open the **Usage** tab. **Near memory max** means the memory is close to the limit. Raise **Maximum memory** the same way. Dify forwards requests to `dify-api` and `dify-web`, so check these two services and `dify-worker` the same way. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

Without Zeabur, read the logs with `kubectl -n <namespace> logs deployment/dify` and the events with `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur docs.
