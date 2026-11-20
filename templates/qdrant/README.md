## Introduction

[Qdrant](https://qdrant.tech/) is a vector database with a REST API, a gRPC API and a web dashboard.

This template runs one Qdrant instance behind your domain, with its data on a volume. Every request needs an API key that is generated at deploy time.

## Getting started

1. Wait until the service shows as running, then note its domain, for example `https://qdrant.example.com`.
2. Get the API key. It is in Secret `qdrant-secrets`, key `api-key`:

   ```sh
   kubectl -n <namespace> get secret qdrant-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

3. Open `https://<your domain>/dashboard` and enter the API key.
4. Send the same key in the `api-key` header on every REST call:

   ```sh
   curl https://<your domain>/collections --header 'api-key: <api-key>'
   ```

5. The gRPC port (`6334`) is not behind the domain. Pods in the same project that carry the label `templates.zeabur.com/qdrant-client=true` reach REST at `qdrant:6333` and gRPC at `qdrant:6334`.

## Configuration

### Connect a client

Pass the API key to the client. For Python:

```python
from qdrant_client import QdrantClient

client = QdrantClient(url="https://<your domain>:443", api_key="<api-key>")
```

See [Security](https://qdrant.tech/documentation/security/) in the Qdrant docs.

### Change the API key

Write a new value to `qdrant-secrets`, key `api-key`, then restart the StatefulSet so it reads the new value:

```sh
kubectl -n <namespace> rollout restart statefulset/qdrant
```

Update every client that uses the old key.
