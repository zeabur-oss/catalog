---
default: minor
---

# Generated inputs state their length and encoding

`generate` on a component or template input is an object: `{type: random, bytes, encoding}` or `{type: clusterIP}`. `bytes` and `encoding` are required, so components and templates no longer have different hidden defaults (64 hex and 32 base64url). `generateBytes` and `generateEncoding` are removed. Input names are camelCase on components too, such as `clusterIp` on Zeabur Builder and `s3Url` on Velero.
