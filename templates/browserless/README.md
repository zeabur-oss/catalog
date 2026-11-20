## Introduction

[Browserless](https://github.com/browserless/browserless) runs headless Chrome as a service. Puppeteer, Playwright and other automation clients connect to it over WebSocket or HTTP, so they do not need a browser on their own machine.

This template runs one Browserless instance behind your domain. Every call needs a token that is generated at deploy time.

## Getting started

1. In Zeabur, open the `browserless` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `browserless.example.com`.
2. Get the token. Open **Variables**, click `browserless-secrets` under **From config files and secrets**, and click the eye icon next to `token`. Without Zeabur, read Secret `browserless-secrets`, key `token`, with kubectl:

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

On the service's **Variables** tab, click `browserless-secrets`, enter a new value for `token`, then review and apply. The running replica keeps the old token until you click **Restart** at the top of the service page. Update every client that uses the old token.

## Troubleshooting

### Sessions wait in a queue or time out

The template allows a limited number of browser sessions at once (`CONCURRENT`) and queues the rest (`QUEUED`). Close browsers your client no longer needs, or raise the memory limit of the service before you raise these values. See [Docker configuration](https://docs.browserless.io/baas/docker/config).

### The page shows 502 or does not load

Open the `browserless` service in Zeabur. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the service is slow or keeps reconnecting, open the **Usage** tab. **Near memory max** means the memory is close to the limit. Raise **Maximum memory** the same way. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

Without Zeabur, read the logs with `kubectl -n <namespace> logs deployment/browserless` and the events with `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur docs.
