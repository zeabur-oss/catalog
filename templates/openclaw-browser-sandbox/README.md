## Introduction

This template runs a headless Chrome that [OpenClaw](https://docs.openclaw.ai/tools/browser/remote) or another service drives through the Chrome DevTools protocol (CDP).

The browser is for one trusted owner. It has no web page, no sign-in and no token. Only pods in the same project that carry the trusted caller label can reach it.

## Getting started

1. In Zeabur, open the `openclaw-sandbox-browser` service. When **Overview** shows its replica as ready, the service is up. It has no domain.
2. Make sure your client Pod carries both of these labels. The first value is the trusted caller you entered in the install form:

   ```yaml
   app.kubernetes.io/name: <trusted caller>
   templates.zeabur.com/template: openclaw-browser-sandbox
   ```

   Pods of the OpenClaw template carry `templates.zeabur.com/template: openclaw`, so they cannot connect yet ([#392](https://github.com/zeabur-oss/catalog/issues/392)).
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

### The client cannot connect

A network policy lets in only pods with the trusted caller label, and only on port 9222. Check that the client runs in the same project and carries both labels from Getting started. See [Remote and hosted browsers](https://docs.openclaw.ai/tools/browser/remote).
