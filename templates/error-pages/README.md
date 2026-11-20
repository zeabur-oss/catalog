## Introduction

Error pages serves a maintenance page and the pages for the status codes 404, 502, 503 and 504. It is a small [nginx](https://nginx.org/) server that holds the pages in a ConfigMap.

This template has no domain and no sign-in. Zeabur sends the maintenance and error pages of the project's domains to this service. Other workloads in the same namespace can use it too.

## Getting started

1. In Zeabur, open the `error-pages` service. When **Overview** shows its replica as ready, the service is up. It has no public domain. The **Networking** tab shows its private address.
2. Other workloads reach it through the Service `error-pages`. Port 8080 serves each page at `http://error-pages:8080/<status>.html`, for example `http://error-pages:8080/502.html`. Port 8081 answers every request with the maintenance page and status `503`.
3. To look at a page from your own machine, open a tunnel and request it. Zeabur has no screen for this, so use kubectl:

   ```sh
   kubectl -n <namespace> port-forward service/error-pages 8080:8080
   curl http://localhost:8080/502.html
   ```

There are no secrets to read for this template.

## Configuration

### Use the pages in a Traefik errors middleware

Create a `Middleware` in the same namespace that sends error responses to port 8080 and asks for the page of the status code:

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: error-pages
spec:
  errors:
    status:
      - "404"
      - "502-504"
    query: /{status}.html
    service:
      name: error-pages
      port: 8080
```

See [Errors](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/errorpages/) in the Traefik docs.

### Change the text of a page

The pages are the keys `404.html`, `502.html`, `503.html`, `504.html` and `maintenance.html` of the ConfigMap `error-pages`. In the project, open the **Objects** page, open the ConfigMap `error-pages`, switch to the YAML view, click **Edit**, change the page, then review and apply. Kubernetes updates the mounted files within about a minute, and nginx reads each page on every request, so you do not need to restart anything.
