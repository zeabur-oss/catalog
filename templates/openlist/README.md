## Introduction

[OpenList](https://github.com/OpenListTeam/OpenList) lists files from many storage providers in one web page and serves them over WebDAV. This template uses the all-in-one image, which includes FFmpeg and Aria2.

This template runs one OpenList instance behind your domain. The administrator password and the Aria2 password are generated at deploy time.

## Getting started

1. Wait until the service shows as running, then open its domain, for example `https://openlist.example.com`.
2. Sign in as `admin`. The password is in Secret `openlist-secrets`, key `admin-password`:

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   OpenList sets this password again at every restart, so a password changed in the web page is lost. To change it, edit the Secret and restart (see Configuration).
3. For offline downloads, the Aria2 token is in the same Secret, key `aria2-password`:

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.aria2-password}' | base64 -d
   ```

   In the Aria2 settings, enter `http://127.0.0.1:6800/jsonrpc` as the endpoint and this value as the token.

## Configuration

### Change the administrator password

Write a new value to `openlist-secrets`, key `admin-password`, then restart the StatefulSet so it reads the new value:

```sh
kubectl -n <namespace> rollout restart statefulset/openlist
```

### Use offline downloads

Open the management page, go to Settings, then Other, and set a temporary folder. Then enter the Aria2 endpoint and token from Getting started. See [Offline download](https://doc.oplist.org/guide/advanced/offline-download).

### Protect a folder with a password

Add a meta record for the path and set its password. Check "Apply to sub folder" if the password must cover subfolders. The password does not apply to WebDAV access. See [Meta](https://doc.oplist.org/guide/advanced/meta).
