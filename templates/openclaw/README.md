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

### The page shows 502 or does not load

In Zeabur, open the `openclaw` service. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica keeps restarting, open **Deployments**, then **Events**. If the events show `OOMKilled`, raise **Maximum memory** in **Settings** → **Compute**. See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/).

If the service is slow or keeps reconnecting, check the same things.

If the domain or its certificate stays pending, see [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting).

### Another browser asks for pairing

Each browser profile pairs on its own, and one profile cannot reuse the approval of another. Approve each profile once with `devices approve` in the **Terminal**, and remove old entries with `devices remove`. See [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair) in the OpenClaw docs.

### All models failed or the provider returns an error

OpenClaw sends model requests to the base URL and model from the install form. Check these:

- The model name exists at your provider. A wrong name is a common cause.
- The API key in the Secret you named in the install form (key `api-key`) is valid and has not hit a rate limit or a quota at the provider.
- The base URL is the HTTPS address of an OpenAI-compatible API.

Open the **Logs** tab of the `openclaw` service to read the error that the provider returned. The base URL and the model are read from the install form on the first start only. To change them later, use the **Config** tab of the Control UI. The base URL is `models.providers.custom.baseUrl`. The model name is set in `models.providers.custom.models` and in `agents.defaults.model.primary`. See [Model providers](https://docs.openclaw.ai/concepts/model-providers) and [Configuration](https://docs.openclaw.ai/gateway/configuration) in the OpenClaw docs.
