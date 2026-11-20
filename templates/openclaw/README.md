## Introduction

[OpenClaw](https://github.com/openclaw/openclaw) is a personal AI assistant. A Gateway runs the assistant, and a web Control UI lets you chat with it and manage it. It can also connect to chat apps such as WhatsApp, Telegram, Slack and Discord.

This template runs one Gateway behind your domain. It uses the OpenAI-compatible provider that you entered in the install form.

## Getting started

1. In Zeabur, open the `openclaw` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `openclaw.example.com`.
2. The sign-in screen has a Gateway secret field. Paste the gateway token into it. Get the token from the `openclaw` service: open **Variables**, click `openclaw-secrets` under **From config files and secrets**, and click the eye icon next to `gateway-token`. Without Zeabur, read Secret `openclaw-secrets`, key `gateway-token`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret openclaw-secrets -o jsonpath='{.data.gateway-token}' | base64 -d
   ```

3. A new browser shows `disconnected (1008): pairing required` until you approve it once. Keep the page open. Click **Terminal** at the top of the `openclaw` service page, list the requests, and approve yours. The page connects by itself after you approve:

   ```sh
   node /app/openclaw.mjs devices list
   node /app/openclaw.mjs devices approve <requestId>
   ```

4. The model provider is the base URL, the model and the API key from the install form. The API key is in the Secret that you named in the form, key `api-key`. It is not in `openclaw-secrets`. To see it, open **Variables** on the `openclaw` service and click that Secret.

## Configuration

### Change the provider API key

On the `openclaw` service, open **Variables** and click the Secret you named in the install form. Enter a new value for `api-key`, review and apply, then click **Restart** at the top of the service page so the Gateway reads the new value.

The base URL and the model are written to the Gateway's configuration file on the first start only. Changing the install form later does not change them.

### Remove a paired browser

Click **Terminal** at the top of the `openclaw` service page. List the paired devices, then remove the one you no longer use:

```sh
node /app/openclaw.mjs devices list
node /app/openclaw.mjs devices remove <deviceId>
```

See [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair) in the OpenClaw docs.

## Troubleshooting

### Another browser asks for pairing

Each browser profile pairs on its own, and one profile cannot reuse the approval of another. Approve each profile once with `devices approve` in the **Terminal**, and remove old entries with `devices remove`. See [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair) in the OpenClaw docs.
