---
default: major
---

# Templates say how to start using them and link their docs

`metadata.docs` links the upstream usage docs, and `metadata.readme` is one short paragraph on what you get, where to open it, how the first person signs in and which Secret holds each generated value, with `zh-TW` and `zh-CN` text in `metadata.locales`. Both are required: a template without them fails `catalog validate`, which also checks that `readme` names the Secret of every `generate` input.
