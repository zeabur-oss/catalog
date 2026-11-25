## Introduction

[SearXNG](https://github.com/searxng/searxng) is a metasearch engine. It sends your query to many search engines and shows the results in one list, without an account or tracking by SearXNG itself.

This template runs one SearXNG instance behind your domain. HTML and JSON results are both on.

## Getting started

1. In Zeabur OSS, open the `searxng` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**; it is the domain you entered in the install form.
2. Search. There is no account and no sign-in, so anyone who knows the address can use it.
3. The signing key of the server is in Secret `app-secrets`, key `signing-key`. You do not need it to search. Open **Variables**, click `app-secrets` under **From config files and secrets**, and click the eye icon next to `signing-key`. Without Zeabur OSS, read Secret `app-secrets`, key `signing-key`, with kubectl:

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

### The page shows 502 or does not load

In Zeabur OSS, open the `searxng` service. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica keeps restarting, open **Deployments**, then **Events**. If the events show `OOMKilled`, raise **Maximum memory** in **Settings** → **Compute**. See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/).

If the domain or its certificate stays pending, see [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting).

### A request with format=csv or format=rss returns 403

This template turns on only the `html` and `json` formats. A format that is not turned on returns `403 Forbidden`. See [Search API](https://docs.searxng.org/dev/search_api.html).
