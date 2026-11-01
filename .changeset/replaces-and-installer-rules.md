---
default: minor
---

# Components can list the ids they replace, and `conformance/select/` gives cases for how installers choose components

`replaces` on a component lists the ids of removed components it took over. `builds/zeabur-builder` replaces `container-registry/zot`. An installer reads an install record with the old id as having the new component. The new reference page "How installers choose and install components" gives the rules, and `bundle.tar.gz` now has `conformance/select/`, with one case per rule in the format of `schema/v1/select-case.schema.json`.
