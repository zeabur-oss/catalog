## Introduction

[LiteLLM](https://github.com/BerriAI/litellm) is a proxy that gives you one OpenAI-compatible API in front of many LLM providers. It also has an admin page where you manage models, keys and spend.

This template runs LiteLLM behind your domain, with models, keys and spend stored in PostgreSQL.

## Getting started

1. In Zeabur, open the `litellm` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `litellm.example.com`. The admin page is at `/ui` on that domain.
2. Get the master key. Open **Variables**, click `litellm-secrets` under **From config files and secrets**, and click the eye icon next to `master-key`. Without Zeabur, read Secret `litellm-secrets`, key `master-key`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret litellm-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

   The key you use is `sk-` followed by this value.
3. Sign in as `admin`. The password is `sk-` followed by the value you just read.
4. The same `sk-` key is the admin API key for requests to `https://litellm.example.com`. Add models and client keys in the admin page.

The key `salt-key` in `litellm-secrets` encrypts the provider credentials in the database. Never change it after you add a model.

## Configuration

### Add a model

In the admin page, open Models + Endpoints and add the model with your provider and its API key. Models you add in the page are stored in the database, so you do not need to edit files or restart. See [Model Management](https://docs.litellm.ai/docs/proxy/model_management).

### Create a key for a client

Do not give the master key to applications. Create a virtual key for each client in the admin page, then point the client at `https://litellm.example.com/v1` and use that key. See [Virtual Keys](https://docs.litellm.ai/docs/proxy/virtual_keys) and [Langchain, OpenAI SDK and other clients](https://docs.litellm.ai/docs/proxy/user_keys).
