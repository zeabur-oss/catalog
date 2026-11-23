---
default: minor
---

# A link for every LicenseRef- licence

Component and template metadata have `licenseUrls`: the HTTPS URL of the text of each `LicenseRef-` licence in `license`, by its id. Every `LicenseRef-` id needs one, and the field names no id that `license` does not use. API v1 carries the field, so a client can link the licence. The n8n templates now spell their licence `LicenseRef-n8n-sustainable-use` in every template.
