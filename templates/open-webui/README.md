## Introduction

[Open WebUI](https://github.com/open-webui/open-webui) is a chat interface for large language models. You connect it to a model provider, and people chat with the models in the browser.

This template runs Open WebUI behind your domain, with a `pipeline` service for pipelines. Sign-up is off, and no model provider is connected yet.

## Getting started

1. In Zeabur, open the `open-webui` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `chat.example.com`.
2. Sign in with the administrator email you entered in the install form. Get the password from the `open-webui` service: open **Variables**, click `app-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `app-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Open WebUI creates the administrator account at the first start, when no user exists. A later change to the Secret does not change the password, so change it in Open WebUI.
4. Sign-up is off. The administrator creates other users.
5. Add a model provider. See Configuration.
6. Secret `app-secrets` also holds two keys that you do not need for daily use. `session-key` is the secret key of the web interface, and `pipeline-key` is the API key that the web interface uses to call the `pipeline` service. Show them the same way on **Variables**. Without Zeabur, use kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.session-key}' | base64 -d
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.pipeline-key}' | base64 -d
   ```

## Configuration

### Add a model provider

Sign in as the administrator, open Settings, then Connections (the admin section). Next to Manage OpenAI API Connections, select Add Connection. Enter the URL of the provider's OpenAI-compatible API and its API key, then save. For OpenAI the URL is `https://api.openai.com/v1`. See [OpenAI](https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai) in the Open WebUI docs.

The template starts Open WebUI with `ENABLE_PERSISTENT_CONFIG` set to `false`. See Troubleshooting for what this means for settings you change.

## Troubleshooting

### The page shows 502, does not load or keeps reconnecting

Open the `open-webui` service in Zeabur. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. A service that is slow or keeps reconnecting is often short of memory too, so check **Events** and **Maximum memory** for it as well. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur docs.

### Settings you changed in the admin panel are gone after a restart

With `ENABLE_PERSISTENT_CONFIG` set to `false`, Open WebUI reads its settings from its environment variables at every start. It lets you edit settings in the admin panel, but does not save the changes. Make the change again after a restart. See [Environment variable configuration](https://docs.openwebui.com/reference/env-configuration) in the Open WebUI docs.
