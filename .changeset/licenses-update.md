---
default: patch
---

# licenses-update regenerates the licence list

`config/licenses.yaml` named a `licenses-update` command that did not exist. `zeabur-catalog licenses-update` now writes the list: `curl -sSL https://spdx.org/licenses/licenses.json | cargo run -p zeabur-catalog -- licenses-update`. On SPDX License List 3.29.0 it writes the committed list.
