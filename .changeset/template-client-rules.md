---
default: minor
---

# `conformance/template/` gives cases for how clients render templates

The new reference page "How clients render templates" gives the checks before rendering, the labels every object gets, how a client tells whether what a template requires is installed, and which language it shows. `bundle.tar.gz` now has `conformance/template/`, with one case per rule in the format of `schema/v1/template-case.schema.json`. `catalog template render` names each failed check by a code, and a missing cluster issuer is now its own error.
