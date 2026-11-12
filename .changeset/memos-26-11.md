---
default: patch
---

# Memos moves to 26.11.0

The Memos template now creates the administrator and turns off sign-up from the files in `/etc/memos/config`, because Memos 26.10.0 and later no longer read `/etc/secrets` and have removed the `/api/v1` routes.
