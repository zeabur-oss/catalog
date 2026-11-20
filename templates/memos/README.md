## Introduction

[Memos](https://usememos.com/) is a note-taking app for short notes, with tags, search and an API. It keeps your notes on your own server.

This template runs one Memos instance behind your domain. It stores data in SQLite on a 10 GiB volume. Sign-up is off, and an administrator account named `admin` is created at the first start.

## Getting started

1. In Zeabur, open the `memos` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `memos.example.com`.
2. Sign in as `admin`. Get the password from the `memos` service: open **Variables**, click `memos-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `memos-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret memos-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Change the password in Memos after you sign in. Memos reads the Secret only when it creates the account, so a later change to the Secret does not change the password.
4. Sign-up is off. Create other accounts as `admin` in the Memos settings.

## Configuration

### Create a personal access token

Create a token in your user settings in Memos, then send it as a Bearer token. Tokens start with `memos_pat_`, and Memos shows the value only once. Use one token for each app, so you can revoke one without breaking the others.

```sh
curl -H "Authorization: Bearer memos_pat_..." https://memos.example.com/api/memos
```

See [API Access](https://usememos.com/docs/integrations/api-access) in the Memos docs.

## Troubleshooting

### A setting is read-only or cannot be saved

Memos makes every setting group that a configuration file declares read-only in the app and the API. The template declares the General group in ConfigMap `memos-config`, which includes the sign-up rule and the custom profile. Memos treats the file as the whole group, so a field you leave out returns to its default. To change these settings, open the project's **Objects** page, open ConfigMap `memos-config`, switch to the YAML view, click **Edit**, change the values, then review and apply. Memos reads the file at start, so click **Restart** at the top of the `memos` service page.

See [Declarative configuration](https://usememos.com/docs/configuration/declarative-configuration) in the Memos docs.

### Sign-in fails

Memos treats user names as case-sensitive, so type `admin` in lower case. See [Troubleshooting](https://usememos.com/docs/troubleshooting) in the Memos docs.
