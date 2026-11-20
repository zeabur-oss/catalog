## Introduction

[Browserless](https://github.com/browserless/browserless) runs headless Chrome as a service. Puppeteer, Playwright and other automation clients connect to it over WebSocket or HTTP, so they do not need a browser on their own machine.

This template runs one Browserless instance behind your domain. Every call needs a token that is generated at deploy time.

## Getting started

1. Wait until the service shows as running, then note its domain, for example `https://browserless.example.com`.
2. Get the token. It is in Secret `browserless-secrets`, key `token`:

   ```sh
   kubectl -n <namespace> get secret browserless-secrets -o jsonpath='{.data.token}' | base64 -d
   ```

3. Point your client at the domain and send the token in an `Authorization: Bearer <token>` header. If the client cannot set headers, add `?token=<token>` to the URL. For Puppeteer:

   ```js
   const browser = await puppeteer.connect({
     browserWSEndpoint: "wss://browserless.example.com?token=<token>",
   });
   ```

   There is no web page and no sign-in. Calls without the token get `401`.

## Configuration

### Connect from Playwright

Use `chromium.connectOverCDP("wss://<your domain>?token=<token>")`. See [Playwright connection](https://docs.browserless.io/examples/playwright-connection) in the Browserless docs.

### Change the token

Write a new value to `browserless-secrets`, key `token`, then restart the deployment so it reads the new value:

```sh
kubectl -n <namespace> rollout restart deployment/browserless
```

Update every client that uses the old token.

## Troubleshooting

### Sessions wait in a queue or time out

The template allows a limited number of browser sessions at once (`CONCURRENT`) and queues the rest (`QUEUED`). Close browsers your client no longer needs, or raise the memory limit of the service before you raise these values. See [Docker configuration](https://docs.browserless.io/baas/docker/config).
