## Introduction

[LobeChat](https://github.com/lobehub/lobe-chat) is a chat interface for AI models. It keeps each person's chats in their own browser.

This template runs LobeChat behind your domain. An access code protects the page, and the server uses the OpenAI API key you chose in the install form. There are no user accounts.

## Getting started

1. Wait until the service shows as running, then open its domain, for example `https://chat.example.com`.
2. When LobeChat asks for the access code, enter it. The code is in Secret `lobechat-secrets`, key `access-code`:

   ```sh
   kubectl -n <namespace> get secret lobechat-secrets -o jsonpath='{.data.access-code}' | base64 -d
   ```

3. Start a chat. The chats use the OpenAI key from the Secret you chose in the install form, key `api-key`. Each person's chat history stays in their own browser.

## Configuration

### Change the access code

Write a new value to `lobechat-secrets`, key `access-code`, then restart the deployment so it reads the new value:

```sh
kubectl -n <namespace> rollout restart deployment/lobechat
```

Tell everyone who uses the page the new code. See [ACCESS_CODE](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/basic.mdx).

### Change the OpenAI API key

Write the new key to the Secret you chose in the install form, key `api-key`, then restart the deployment:

```sh
kubectl -n <namespace> rollout restart deployment/lobechat
```

See [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx).
