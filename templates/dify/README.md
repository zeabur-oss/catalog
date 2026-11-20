## Introduction

[Dify](https://github.com/langgenius/dify) is a platform for building LLM apps. You design workflows, agents and chat apps in a web editor, and publish them as web apps or APIs.

This template runs Dify behind your domain, with its API, worker, plugin daemon, code sandbox, PostgreSQL and a queue. Dify keeps documents and uploaded files in the S3-compatible storage you entered in the install form.

## Getting started

1. In Zeabur, open the `dify` service. When **Overview** shows its replicas as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `dify.example.com`. Dify also needs its other services, such as `dify-api`, to be ready.
2. Get the initialization password. Open the `dify-api` service, open **Variables**, click `dify-secrets` under **From config files and secrets**, and click the eye icon next to `init-password`. Without Zeabur, read Secret `dify-secrets`, key `init-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret dify-secrets -o jsonpath='{.data.init-password}' | base64 -d
   ```

3. Open `https://dify.example.com/install`. Dify asks for the initialization password first, so nobody else can claim the instance. Enter it, then create the administrator account.
4. Sign in at `https://dify.example.com` with the account you created.

The other keys in `dify-secrets` (`app-key`, `redis-password` and `sandbox-key`) are for Dify's own services. Do not share them.

## Configuration

### Add a model provider

Dify has no models until you connect a provider. In Dify, open Integrations > Model Provider, install the provider you use, click Add API Key, and enter your key. Dify checks the key before it makes the provider available. See [Model Providers](https://docs.dify.ai/en/use-dify/workspace/model-providers).
