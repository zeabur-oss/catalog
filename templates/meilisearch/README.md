## Introduction

[Meilisearch](https://www.meilisearch.com/) is a search engine with a REST API. You send it documents, and it answers search requests in milliseconds.

This template runs one Meilisearch instance behind your domain, with its indexes on a volume. Every request needs a key. There is no web page and no sign-in.

## Getting started

1. In Zeabur OSS, open the `meilisearch` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `search.example.com`.
2. Get the master key. Open **Variables**, click `meilisearch-secrets` under **From config files and secrets**, and click the eye icon next to `master-key`. Without Zeabur OSS, read Secret `meilisearch-secrets`, key `master-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret meilisearch-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

3. Send requests to the domain with the header `Authorization: Bearer <key>`. For example, list the API keys:

   ```sh
   curl https://search.example.com/keys -H 'Authorization: Bearer <master key>'
   ```

Use the master key only to manage API keys. Give clients a search key.

## Configuration

### Get a key for your clients

Meilisearch creates a Default Search API Key and a Default Admin API Key for you. The `GET /keys` request above lists them. Use the search key in your web or mobile app, and the admin key on your server to add documents. See [Basic security](https://www.meilisearch.com/docs/learn/security/basic_security).

### Create a dump

A dump is a backup of all data of the instance. Create one with an API key that is allowed to do it:

```sh
curl -X POST https://search.example.com/dumps -H 'Authorization: Bearer <key>'
```

Meilisearch writes the dump to `/meili_data/dumps` on the volume. See [Dumps](https://www.meilisearch.com/docs/learn/data_backup/dumps).

## Troubleshooting

### Requests return 502 or time out

Open the `meilisearch` service in Zeabur OSS. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur OSS docs.

### Requests return 401

Meilisearch rejects requests without a valid key. Send the key in the `Authorization: Bearer` header, and check that you use a key of this instance. See [Basic security](https://www.meilisearch.com/docs/learn/security/basic_security).
