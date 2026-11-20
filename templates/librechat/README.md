## Introduction

[LibreChat](https://github.com/danny-avila/LibreChat) is a chat app for AI models. People sign in, talk to models, and search their conversations.

This template runs LibreChat behind your domain, with MongoDB for data, Meilisearch for search, and PostgreSQL with pgvector for document search. Chat and document embeddings use the OpenAI API key you chose in the install form.

## Getting started

1. Wait until all services show as running, then open the domain of the `librechat` service, for example `https://chat.example.com`.
2. The template creates the administrator account at deploy time, from the administrator email you entered in the install form. Get its password. It is in Secret `app-secrets`, key `admin-password`:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Sign in with the administrator email and that password. Public registration is off, so nobody else can sign up. Changing the Secret later does not reset the password of an existing account.

The other keys in `app-secrets` (`mongo-password`, `jwt-secret`, `jwt-refresh-secret`, `creds-key`, `creds-iv` and `meili-key`) are for LibreChat's own services. Keep them for the life of the data.

## Configuration

### Change the OpenAI API key

The key is in the Secret you chose in the install form, key `OPENAI_API_KEY`. Write the new value there, then restart the two deployments that use it:

```sh
kubectl -n <namespace> rollout restart deployment/librechat deployment/rag-api
```

See [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai).
