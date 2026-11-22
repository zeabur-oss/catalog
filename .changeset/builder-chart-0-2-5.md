---
default: patch
---

# Zeabur Builder installs chart 0.2.5

The upload Secrets of Zeabur Builder now hold the registry's CA in `ca.crt`, so an uploader that may read the Secret trusts zot without reading zot's own Secret. The controller may also read ServiceAccounts, which the optional agent builder needs.
