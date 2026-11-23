---
default: minor
---

# Zeabur Builder follows a rotated zot CA without a reinstall

`provides.registry.ca` of `zeabur-builder` now names `builder-system/registry-ca`, a Secret that holds zot's CA and no key. Zeabur Builder keeps it in step with `zot/zot-tls`.
