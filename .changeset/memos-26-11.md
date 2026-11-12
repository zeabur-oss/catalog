---
default: patch
---

# Memos moves to 26.11.0

The Memos template now creates the administrator and turns off sign-up from the files in `/etc/memos/config`, because Memos 26.10.0 stopped reading `/etc/secrets`. The ConfigMap `memos-settings` is now `memos-config`, and the postStart hook is gone. Memos 26.10.0 and later also remove the `/api/v1` routes and keep the stored upload size and memo length limits at their defaults, so set the limits again under Settings, Usage limits if you changed them. Upgrade from Memos 0.31.0 only, since older versions have to go through 0.31.0 first.
