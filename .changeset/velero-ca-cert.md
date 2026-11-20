---
default: minor
---

# Velero trusts a store's own CA

Velero has a new input, `caCert`, for the CA certificate of an S3 store that serves HTTPS with its own certificate, such as a self-hosted MinIO. Leave it empty for a store with a public certificate. The `s3Url` description now says the URL must use https, because the store encrypts each backup with the backup key and S3 servers refuse that encryption over plain HTTP.
