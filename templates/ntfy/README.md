## Introduction

[ntfy](https://ntfy.sh/) sends push notifications to your phone or desktop from scripts and apps, through plain HTTP requests. You publish a message to a topic, and every client that subscribes to the topic gets it.

This template runs one ntfy server behind your domain. Every topic needs an account, and messages and users are kept on a 5 GiB volume. A user named `admin` is created at every start.

## Getting started

1. Wait until the service shows as running, then open its domain, for example `https://ntfy.example.com`.
2. Sign in as `admin`. The password is in Secret `ntfy-secrets`, key `admin-password`:

   ```sh
   kubectl -n <namespace> get secret ntfy-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Every topic denies access by default, so an app that publishes or subscribes must sign in with this account or with an access token. See Configuration.

## Configuration

### Change the admin password

ntfy applies the Secret at every start. To change the password, write a new value to `ntfy-secrets`, key `admin-password`, then restart the StatefulSet:

```sh
kubectl -n <namespace> patch secret ntfy-secrets --type merge -p '{"stringData":{"admin-password":"<new password>"}}'
kubectl -n <namespace> rollout restart statefulset/ntfy
```

### Add a user

Run `ntfy user add` in the Pod, then give the user access to a topic with `ntfy access`. `rw` means the user can publish and subscribe:

```sh
kubectl -n <namespace> exec -it statefulset/ntfy -- ntfy user add <username>
kubectl -n <namespace> exec statefulset/ntfy -- ntfy access <username> <topic> rw
```

See [Access control](https://docs.ntfy.sh/config/) in the ntfy docs.

### Create an access token

An access token lets an app publish or subscribe without your password. Create one in the web app, in the Account section after you sign in. You can also create one in the Pod:

```sh
kubectl -n <namespace> exec statefulset/ntfy -- ntfy token add --label=<label> admin
```

Send the token as a Bearer token:

```sh
curl -H "Authorization: Bearer tk_..." -d "Hello" https://ntfy.example.com/mytopic
```

See [Publishing](https://docs.ntfy.sh/publish/) in the ntfy docs.

## Troubleshooting

### A client cannot publish or subscribe

The template denies access to every topic by default. Make the client sign in with a user name and password or an access token. For a user other than `admin`, also run `ntfy access` to give it access to the topic. See [Access control](https://docs.ntfy.sh/config/) in the ntfy docs.
