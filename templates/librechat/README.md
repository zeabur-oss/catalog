## Introduction

[LibreChat](https://github.com/danny-avila/LibreChat) is a chat app for AI models. People sign in, talk to models, and search their conversations.

This template runs LibreChat behind your domain, with MongoDB for data, Meilisearch for search, and PostgreSQL with pgvector for document search. Chat and document embeddings use the OpenAI API key you chose in the install form.

## Getting started

1. In Zeabur, open the `librechat` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `chat.example.com`.
2. The template creates the administrator account at deploy time, from the administrator email you entered in the install form. Get its password. Open **Variables**, click `app-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `app-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Sign in with the administrator email and that password. Public registration is off, so nobody else can sign up. Changing the Secret later does not reset the password of an existing account.

The other keys in `app-secrets` (`mongo-password`, `jwt-secret`, `jwt-refresh-secret`, `creds-key`, `creds-iv` and `meili-key`) are for LibreChat's own services. Keep them for the life of the data.

## Configuration

### Change the OpenAI API key

The key is in the Secret you chose in the install form, key `OPENAI_API_KEY`. On the `librechat` service's **Variables** tab, click that Secret under **From config files and secrets**, enter a new value, then review and apply. Running replicas keep the old key, so click **Restart** at the top of the `librechat` service page and of the `rag-api` service page.

See [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai).
