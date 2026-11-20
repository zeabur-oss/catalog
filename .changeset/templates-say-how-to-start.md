---
default: minor
---

# Templates have READMEs and link their docs

`metadata.docs` links the upstream usage docs. Each template has `README.md`, `README.zh-TW.md` and `README.zh-CN.md` with an Introduction, numbered Getting started steps (where to open the app, how the first person signs in, which Secret holds each generated value), and optional Configuration and Troubleshooting sections. `catalog validate` checks that the files and these headings exist and that Getting started names the Secret of every `generate` input.
