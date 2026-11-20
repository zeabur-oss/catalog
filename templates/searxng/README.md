## Introduction

[SearXNG](https://github.com/searxng/searxng) is a metasearch engine. It sends your query to many search engines and shows the results in one list, without an account or tracking by SearXNG itself.

This template runs one SearXNG instance behind your domain. HTML and JSON results are both on.

## Getting started

1. Wait until the service shows as running, then open its domain, for example `https://searxng.example.com`.
2. Search. There is no account and no sign-in, so anyone who knows the address can use it.
3. The signing key of the server is in Secret `app-secrets`, key `signing-key`. You do not need it to search:

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.signing-key}' | base64 -d
   ```

## Configuration

### Get results as JSON

Add `format=json` to a search request:

```sh
curl 'https://<your domain>/search?q=searxng&format=json'
```

See [Search API](https://docs.searxng.org/dev/search_api.html).

## Troubleshooting

### A request with format=csv or format=rss returns 403

This template turns on only the `html` and `json` formats. A format that is not turned on returns `403 Forbidden`. See [Search API](https://docs.searxng.org/dev/search_api.html).
