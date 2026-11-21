## Introduction

This template runs a headless Chrome that [OpenClaw](https://docs.openclaw.ai/tools/browser/remote) or another service drives through the Chrome DevTools protocol (CDP).

The browser is for one trusted owner. It has no web page, no sign-in and no token. Only pods in the same project that carry the label `templates.zeabur.com/openclaw-browser-sandbox-client: "true"` can reach it.

## Getting started

1. In Zeabur OSS, open the `openclaw-sandbox-browser` service. When **Overview** shows its replica as ready, the service is up. It has no domain.
2. Pods of the OpenClaw template already carry the client label. Any other client Pod needs it in its Pod template:

   ```yaml
   templates.zeabur.com/openclaw-browser-sandbox-client: "true"
   ```
3. From that pod, use this address as the CDP endpoint:

   ```text
   http://openclaw-sandbox-browser:9222
   ```

   Pods without the label are blocked. There is no generated Secret, so anything allowed to connect controls the browser.

## Configuration

### Connect OpenClaw

Set `browser.profiles.<name>.cdpUrl` to the address above and make that profile the default:

```json5
{
  browser: {
    enabled: true,
    defaultProfile: "sandbox",
    profiles: {
      sandbox: { cdpUrl: "http://openclaw-sandbox-browser:9222" },
    },
  },
}
```

With an `http://` address, OpenClaw finds the WebSocket endpoint through `/json/version`. See [Remote and hosted browsers](https://docs.openclaw.ai/tools/browser/remote).

## Troubleshooting

### The browser restarts or stops answering

In Zeabur OSS, open the `openclaw-sandbox-browser` service. Check that **Overview** shows its replica as ready, then read the **Logs** tab. Open **Deployments**, then **Events**. If the events show `OOMKilled`, raise **Maximum memory** in **Settings** → **Compute**. See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/).

### The client cannot connect

A network policy lets in only pods with the client label, and only on port 9222. Check that the client runs in the same project and carries the label from Getting started. See [Remote and hosted browsers](https://docs.openclaw.ai/tools/browser/remote).
