---
default: minor
---

# Add the zeabur-catalog-format Rust crate

`crates/zeabur-catalog-format` holds the typed model of component, template, API and measurement files and the rules installers follow: install order, choosing components, contributions and release values. Readers of the API ignore fields and values they do not know, and the catalog's check refuses them by path. A test runs every `conformance/select/` case against it.
