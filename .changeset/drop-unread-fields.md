---
default: major
---

# The catalog format drops the fields no client reads

Components lose `maturity`, `stability`, `alternatives`, `disruptsTraffic` and `upgradeNotes`; their `security` keeps only `summary`, `privileged` and `podSecurity`. Templates lose `metadata.icon`, `metadata.categories`, `scalability`, `upgradeNotes` and `services[].baselineJustification`. `index.json` no longer has `nonOSILicenses`, `capabilities`, `budget` or `templates`. Components and templates also lose `metadata.description`; its text is now a comment at the top of the file. A file with one of these fields fails validation.

Before 1.0, a minor release can change the file format, so pin an exact release.
