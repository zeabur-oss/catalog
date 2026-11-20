## Introduction

[New API](https://github.com/QuantumNous/new-api) is a gateway for AI models. It gives your apps one OpenAI-compatible API and sends each request to the model provider you set up.

This template runs one New API behind your domain, with its own PostgreSQL database. The first-time setup is done for you, so no setup page appears, and an administrator account named `admin` exists.

## Getting started

1. In Zeabur, open the `new-api` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `newapi.example.com`.
2. Sign in as `admin`. The password is `Aa1-` followed by the value of key `admin-password`. Get the value from the `new-api` service: open **Variables**, click `newapi-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Add `Aa1-` in front of it. Without Zeabur, this kubectl command prints the whole password:

   ```sh
   echo "Aa1-$(kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.admin-password}' | base64 -d)"
   ```

3. Add your model providers as channels. See Configuration.
4. Secret `newapi-secrets` also holds two keys that you do not need for daily use. `session-secret` is the session secret, and `crypto-secret` is the HMAC secret for cache keys. Show them the same way on **Variables**. Without Zeabur, use kubectl:

   ```sh
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.session-secret}' | base64 -d
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.crypto-secret}' | base64 -d
   ```

## Configuration

### Add a model provider

A channel is one connection to a model provider. Sign in as `admin`, open Channels in the left sidebar, and select Add Channel. Choose the provider type, enter a name and the provider's API key, and choose the models the channel supports. Use Test on the channel to check that it works. See [Channel Management](https://docs.newapi.ai/en/docs/guide/feature-guide/admin/channel) in the New API docs.

### Call the gateway from an app

Create an API token in the console. See [API Tokens](https://docs.newapi.ai/en/docs/guide/console/api-token). Then point your app at `https://<your domain>/v1` and send the token as a Bearer token:

```sh
curl https://newapi.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-<token>" \
  -d '{"model":"<model>","messages":[{"role":"user","content":"Hello"}]}'
```

See [Using API and Playground](https://docs.newapi.ai/en/docs/guide/feature-guide/user/api) in the New API docs.

## Troubleshooting

### Sign-in fails with the generated password

The password is not the Secret value alone. It starts with `Aa1-`. Use the command in Getting started to print it.
