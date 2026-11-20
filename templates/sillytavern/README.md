## Introduction

[SillyTavern](https://github.com/SillyTavern/SillyTavern) is a chat front end for large language models. It connects to the model APIs you choose and keeps your characters and chats.

This template runs one SillyTavern instance behind your domain. Access is protected by one Basic authentication password that is generated at deploy time. Chats and settings are kept on a volume.

## Getting started

1. In Zeabur, open the `sillytavern` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**; it is the domain you entered in the install form.
2. Your browser asks for a login. The user name is `user`. The password is in Secret `app-secrets`, key `basic-password`. Open **Variables**, click `app-secrets` under **From config files and secrets**, and click the eye icon next to `basic-password`. Without Zeabur, read Secret `app-secrets`, key `basic-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.basic-password}' | base64 -d
   ```

3. There are no separate user accounts. After you sign in, add your model API connection (see Configuration).

## Configuration

### Connect a model API

In the top bar, click API Connections, choose the API type and source, enter your API key and click Connect. See [Quick Start](https://docs.sillytavern.app/usage/quick-start/) and [API Connections](https://docs.sillytavern.app/usage/api-connections/).

### Change the password

On the service's **Variables** tab, click `app-secrets`, enter a new value for `basic-password`, then review and apply. The running replica keeps the old value until you click **Restart** at the top of the service page. Your browser asks for the new password the next time you open the page.

## Troubleshooting

### The browser keeps asking for a login

Use `user` as the user name, and the password from `app-secrets`, key `basic-password`. After a password change, the old saved password no longer works.
