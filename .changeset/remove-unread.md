---
default: major
---

# Remove the parts of the catalog that nothing reads

- `types/v1/catalog.ts` and the TypeScript half of `zeabur-catalog schema` are removed. The JSON Schemas under `schema/` stay.
- The `utoipa` feature of `zeabur-catalog-format` is removed.
- `zeabur-catalog licenses-update` is removed. `config/licenses.yaml` says how to refresh it.
- `zeabur-catalog template render` and `template inputs` are removed. Deploy a template with `zeaburctl templates export`.
- The `.#catalog` Go package and its vendor-hash workflow are removed. The release reads its version from `.#zeabur-catalog`.
- `catalog e2e -tier check` is removed. `scripts/ci/check.sh` runs `zeabur-catalog check`, `render` and `dry-run`.
- The `zeabur-catalog` command line is parsed with clap, so `--help` lists every command.
