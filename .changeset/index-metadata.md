---
default: minor
---

# The index names its release, and locales take any language

`index.json` has `catalog` with the `version` of the release that built it. The keys of every `locales` map are BCP 47 tags, and `config/locales.yaml` lists the languages each map must have, now `zh-TW` and `zh-CN`. The release docs give the rule for each value a reader doesn't know, such as an unknown release `type` or `generate.type`.
