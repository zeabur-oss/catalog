## Introduction

[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) is an API proxy. Clients send OpenAI-style or Gemini-style requests to it, and it forwards them to the model providers you set up.

This template runs one CLIProxyAPI instance behind your domain. The API is protected by a key that is generated at deploy time. There is no web page and no sign-in. The management API is not on the domain.

## Getting started

1. Wait until the service shows as running, then note its domain, for example `https://cliproxyapi.example.com`.
2. Get the API key. It is in Secret `app-secrets`, key `api-key`:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

3. Point your client at `https://cliproxyapi.example.com/v1` (or `/v1beta` for Gemini-style clients) and send the key as a Bearer token. Calls without it get `401`.
4. Get the management key when you need it. It is in Secret `app-secrets`, key `management-key`:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.management-key}' | base64 -d
   ```

The API key only lets clients in. Credentials for the model providers are separate; see Configuration.

## Configuration

### Call the management API

The management API only accepts calls from inside the pod. Open a tunnel with `kubectl`, then send the management key as a Bearer token:

```sh
kubectl -n <namespace> port-forward deployment/cliproxyapi 8317:8317 --address 127.0.0.1
curl -H 'Authorization: Bearer <management key>' http://localhost:8317/v8/management/config
```

See [Management API v8](https://help.router-for.me/management/apiv8.html).

### Add a provider credential

Upload a credential file that the CLIProxyAPI login command created. With the tunnel from the previous item open:

```sh
curl -X POST -H 'Authorization: Bearer <management key>' \
  -F file=@<credential>.json \
  http://localhost:8317/v8/management/credentials
```

The file is stored on the volume and survives restarts. See [Management API v8](https://help.router-for.me/management/apiv8.html) and [Authentication Directory](https://help.router-for.me/configuration/auth-dir.html).

Provider keys that you write through `/config` are kept only until the pod restarts, because the template builds the configuration file again on every start.

## Troubleshooting

### The management API returns 401 or stops answering

Five wrong keys in a row from one address ban that address for about 30 minutes. Check that you use the value of `management-key`, not `api-key`, and wait for the ban to end. See [Management API v8](https://help.router-for.me/management/apiv8.html).
