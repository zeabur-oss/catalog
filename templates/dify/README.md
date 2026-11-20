## Introduction

[Dify](https://github.com/langgenius/dify) is a platform for building LLM apps. You design workflows, agents and chat apps in a web editor, and publish them as web apps or APIs.

This template runs Dify behind your domain, with its API, worker, plugin daemon, code sandbox, PostgreSQL and a queue. Dify keeps documents and uploaded files in the S3-compatible storage you entered in the install form.

## Getting started

1. Wait until all services show as running, then note the domain of the `dify` service, for example `https://dify.example.com`.
2. Get the initialization password. It is in Secret `dify-secrets`, key `init-password`:

   ```sh
   kubectl -n <namespace> get secret dify-secrets -o jsonpath='{.data.init-password}' | base64 -d
   ```

3. Open `https://dify.example.com/install`. Dify asks for the initialization password first, so nobody else can claim the instance. Enter it, then create the administrator account.
4. Sign in at `https://dify.example.com` with the account you created.

The other keys in `dify-secrets` (`app-key`, `redis-password` and `sandbox-key`) are for Dify's own services. Do not share them.

## Configuration

### Add a model provider

Dify has no models until you connect a provider. In Dify, open Integrations > Model Provider, install the provider you use, click Add API Key, and enter your key. Dify checks the key before it makes the provider available. See [Model Providers](https://docs.dify.ai/en/use-dify/workspace/model-providers).
