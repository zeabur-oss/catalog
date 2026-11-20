## Introduction

[OpenClaw](https://github.com/openclaw/openclaw) is a personal AI assistant. A Gateway runs the assistant, and a web Control UI lets you chat with it and manage it. It can also connect to chat apps such as WhatsApp, Telegram, Slack and Discord.

This template runs one Gateway behind your domain. It uses the OpenAI-compatible provider that you entered in the install form.

## Getting started

1. Wait until the service shows as running, then open its domain, for example `https://openclaw.example.com`.
2. The sign-in screen has a Gateway secret field. Paste the gateway token into it. The token is in Secret `openclaw-secrets`, key `gateway-token`:

   ```sh
   kubectl -n <namespace> get secret openclaw-secrets -o jsonpath='{.data.gateway-token}' | base64 -d
   ```

3. A new browser shows `disconnected (1008): pairing required` until you approve it once. Keep the page open, list the requests, and approve yours. The page connects by itself after you approve:

   ```sh
   kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices list
   kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices approve <requestId>
   ```

4. The model provider is the base URL, the model and the API key from the install form. The API key is in the Secret that you named in the form, key `api-key`. It is not in `openclaw-secrets`.

## Configuration

### Change the provider API key

Write the new key to the Secret that you named in the install form, key `api-key`, then restart the Deployment so it reads the new value:

```sh
kubectl -n <namespace> rollout restart deployment/openclaw
```

The base URL and the model are written to the Gateway's configuration file on the first start only. Changing the install form later does not change them.

### Remove a paired browser

List the paired devices, then revoke or remove the one you no longer use:

```sh
kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices list
kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices remove <deviceId>
```

See [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair) in the OpenClaw docs.

## Troubleshooting

### Another browser asks for pairing

Each browser profile pairs on its own, and one profile cannot reuse the approval of another. Approve each profile once with `devices approve`, and remove old entries with `devices remove`. See [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair) in the OpenClaw docs.
