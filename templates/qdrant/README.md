## Introduction

[Qdrant](https://qdrant.tech/) is a vector database with a REST API, a gRPC API and a web dashboard.

This template runs one Qdrant instance behind your domain, with its data on a volume. Every request needs an API key that is generated at deploy time.

## Getting started

1. In Zeabur, open the `qdrant` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**; it is the domain you entered in the install form.
2. Get the API key. Open **Variables**, click `qdrant-secrets` under **From config files and secrets**, and click the eye icon next to `api-key`. Without Zeabur, read Secret `qdrant-secrets`, key `api-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret qdrant-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

3. Open `https://<your domain>/dashboard` and enter the API key.
4. Send the same key in the `api-key` header on every REST call:

   ```sh
   curl https://<your domain>/collections --header 'api-key: <api-key>'
   ```

5. The gRPC port (`6334`) is not behind the domain. Pods in the same project reach REST at `qdrant:6333` and gRPC at `qdrant:6334`.

## Configuration

### Connect a client

Pass the API key to the client. For Python:

```python
from qdrant_client import QdrantClient

client = QdrantClient(url="https://<your domain>:443", api_key="<api-key>")
```

See [Security](https://qdrant.tech/documentation/security/) in the Qdrant docs.

### Change the API key

On the service's **Variables** tab, click `qdrant-secrets`, enter a new value for `api-key`, then review and apply. The running replica keeps the old value until you click **Restart** at the top of the service page. Update every client that uses the old key.

## Troubleshooting

### The page shows 502 or does not load

In Zeabur, open the `qdrant` service. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica keeps restarting, open **Deployments**, then **Events**. If the events show `OOMKilled`, raise **Maximum memory** in **Settings** → **Compute**. See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/).

If the service is slow or keeps reconnecting, check the same things.

If the domain or its certificate stays pending, see [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting).
