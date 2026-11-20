## Introduction

[Hermes Agent](https://github.com/NousResearch/hermes-agent) is an AI agent that works through an API and a web dashboard. The API follows the OpenAI format, so OpenAI-compatible clients can use the agent as a backend.

This template runs Hermes Agent with two domains: one for the API and one for the dashboard. The agent calls the model through OpenRouter or the OpenAI-compatible endpoint you entered in the install form.

## Getting started

1. In Zeabur, open the `hermes-agent` service. When **Overview** shows its replica as ready, the service is up. Both domains are on the **Networking** tab under **Public domains**, for example `hermes.example.com` for the API and `hermes-admin.example.com` for the dashboard.
2. Get the dashboard password. Open **Variables**, click `hermes-secrets` under **From config files and secrets**, and click the eye icon next to `dashboard-password`. Without Zeabur, read Secret `hermes-secrets`, key `dashboard-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret hermes-secrets -o jsonpath='{.data.dashboard-password}' | base64 -d
   ```

3. Open the dashboard domain and sign in as `admin` with that password.
4. Get the API key. In the same `hermes-secrets` list, click the eye icon next to `api-key`. With kubectl:

   ```sh
   kubectl -n <namespace> get secret hermes-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

5. Call the API at `https://hermes.example.com/v1` and send the key as a Bearer token:

   ```sh
   curl https://hermes.example.com/v1/chat/completions \
     -H "Authorization: Bearer <api key>" \
     -H "Content-Type: application/json" \
     -d '{"model": "hermes-agent", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

The key of your model provider is not in `hermes-secrets`. It is in the Secret you chose in the install form, key `api-key`. The key `dashboard-session-key` in `hermes-secrets` signs dashboard sessions.

## Configuration

### Connect a client

Open WebUI, LobeChat, LibreChat and other OpenAI-compatible clients can use the agent. Set the API base URL to `https://hermes.example.com/v1` and the API key to the value of `api-key` in `hermes-secrets`. See [API Server](https://hermes-agent.nousresearch.com/docs/user-guide/features/api-server).

### Change the provider API key

On the service's **Variables** tab, click the Secret you chose in the install form under **From config files and secrets**, enter a new value for `api-key`, then review and apply. The running replica keeps the old key until you click **Restart** at the top of the service page.

The template writes the model, endpoint and key from the install form into the Hermes configuration on every start. A change of the model in the dashboard lasts only until the next restart.

See [Web Dashboard](https://hermes-agent.nousresearch.com/docs/user-guide/features/web-dashboard) for what the dashboard manages.
