---
default: minor
---

# Components have READMEs

Each component has `components/<id>.README.md`, `.README.zh-TW.md` and `.README.zh-CN.md` with an Introduction, Use it and Remove it, and optional Before you install and Troubleshooting sections. Use it links to the Zeabur docs page where the component shows up. `catalog validate` checks the files and headings, and requires Before you install when an input needs a value from outside the cluster.
