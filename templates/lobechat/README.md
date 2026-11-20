## Introduction

[LobeChat](https://github.com/lobehub/lobe-chat) is a chat interface for AI models. It keeps each person's chats in their own browser.

This template runs LobeChat behind your domain. An access code protects the page, and the server uses the OpenAI API key you chose in the install form. There are no user accounts.

## Getting started

1. In Zeabur, open the `lobechat` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `chat.example.com`.
2. When LobeChat asks for the access code, enter it. To read it, open **Variables**, click `lobechat-secrets` under **From config files and secrets**, and click the eye icon next to `access-code`. Without Zeabur, read Secret `lobechat-secrets`, key `access-code`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret lobechat-secrets -o jsonpath='{.data.access-code}' | base64 -d
   ```

3. Start a chat. The chats use the OpenAI key from the Secret you chose in the install form, key `api-key`. Each person's chat history stays in their own browser.

## Configuration

### Change the access code

On the service's **Variables** tab, click `lobechat-secrets`, enter a new value for `access-code`, then review and apply. The running replica keeps the old code until you click **Restart** at the top of the service page. Tell everyone who uses the page the new code. See [ACCESS_CODE](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/basic.mdx).

### Change the OpenAI API key

On the service's **Variables** tab, click the Secret you chose in the install form, enter a new value for `api-key`, then review and apply. Click **Restart** at the top of the service page so the replica reads it.

See [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx).
