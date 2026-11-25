## Introduction

[LibreChat](https://github.com/danny-avila/LibreChat) is a chat app for AI models. People sign in, talk to models, and search their conversations.

This template runs LibreChat behind your domain, with MongoDB for data, Meilisearch for search, and PostgreSQL with pgvector for document search. Chat and document embeddings use the OpenAI API key you chose in the install form.

## Getting started

1. In Zeabur OSS, open the `librechat` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `chat.example.com`.
2. The template creates the administrator account at deploy time, from the administrator email you entered in the install form. Get its password. Open **Variables**, click `app-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur OSS, read Secret `app-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Sign in with the administrator email and that password. Public registration is off, so nobody else can sign up. Changing the Secret later does not reset the password of an existing account.

The other keys in `app-secrets` (`mongo-password`, `jwt-secret`, `jwt-refresh-secret`, `creds-key`, `creds-iv` and `meili-key`) are for LibreChat's own services. Keep them for the life of the data.

## Configuration

### Change the OpenAI API key

The key is in the Secret you chose in the install form, key `OPENAI_API_KEY`. On the `librechat` service's **Variables** tab, click that Secret under **From config files and secrets**, enter a new value, then review and apply. Running replicas keep the old key, so click **Restart** at the top of the `librechat` service page and of the `rag-api` service page.

See [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai).

### Back up the data

LibreChat keeps users and chats in the `mongodb` database, and the document embeddings in the `rag-db` database. In the project, open each of them. Under **Backups**, set up S3 backups. To get a file instead, click **More** → **Export**. Files that people upload are on the volume `librechat-data`, mounted at `/app/uploads`. Copy it with the **Files** tab of the `librechat` service. For a large folder, use kubectl. `kubectl -n <namespace> get pods` lists the replica names:

```sh
kubectl -n <namespace> cp <replica>:/app/uploads ./librechat-uploads
```

Keep the Secret `app-secrets` with the backup, as Getting started says. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump) in the Zeabur OSS docs.

## Troubleshooting

### The page shows 502 or does not load

Open the `librechat` service in Zeabur OSS. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the service is slow or keeps reconnecting, open the **Usage** tab. **Near memory max** means the memory is close to the limit. Raise **Maximum memory** the same way. If chats work but file search fails, check the `rag-api` service the same way. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

Without Zeabur OSS, read the logs with `kubectl -n <namespace> logs deployment/librechat` and the events with `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur OSS docs.

### A chat or a file search fails with a model provider error

Chat and document embeddings use the key `OPENAI_API_KEY` from the Secret you chose in the install form. When a request fails, read the **Logs** tab of the `librechat` service. For a file search, read the `rag-api` service as well. Then check three things. The key must be valid. The model you picked must be available to that key. And your account must not be over its rate limit or quota at OpenAI. See [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai) in the LibreChat docs.
