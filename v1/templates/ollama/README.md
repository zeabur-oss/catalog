## Introduction

[Ollama](https://github.com/ollama/ollama) runs large language models and serves them through an HTTP API.

This template runs one Ollama on the CPU, with a 50 GiB volume for models. It has no domain, no web page and no sign-in. Only Pods you allow, in the same namespace, can reach its API. No model is preloaded.

## Getting started

1. In Zeabur OSS, open the `ollama` service. When **Overview** shows its replica as ready, the service is up. It has no domain.
2. Other workloads reach Ollama at Service `ollama`, port `11434`, for example `http://ollama:11434`. The template allows only Pods in the same namespace that carry this label:

   ```yaml
   templates.zeabur.com/ollama-client: "true"
   ```

3. To use the API from your own computer, forward the port, then use `http://localhost:11434`. Zeabur OSS has no UI for port forwarding, so use kubectl:

   ```sh
   kubectl -n <namespace> port-forward service/ollama 11434
   ```

4. Pull a model. Read the model's license and check its size first. The Pod has an 8 GiB memory limit, so large models do not fit. Click **Terminal** at the top of the `ollama` service page and run:

   ```sh
   ollama pull <model>
   ```

There is no password, so the template creates no Secret.

## Configuration

### Connect an OpenAI client

Ollama has an OpenAI-compatible API. Set the client's base URL to `http://ollama:11434/v1`. The client needs an API key value, but Ollama ignores it, so use any text. See [OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility) in the Ollama docs.

### List and remove models

Click **Terminal** at the top of the `ollama` service page and run:

```sh
ollama ls
ollama rm <model>
```

See [CLI reference](https://docs.ollama.com/cli) in the Ollama docs.

## Troubleshooting

### A connection from a Pod times out

The Pod needs both labels from Getting started. Check the labels of the Pod that fails, and add the missing one to the Pod template of that workload.

### A model is slow or does not load

This template runs on the CPU, with an 8 GiB memory limit and one loaded model at a time. Click **Terminal** at the top of the `ollama` service page and run `ollama ps` to see what is loaded. Choose a smaller model if the model does not fit. The **Logs** tab of the service shows the reason.

```sh
ollama ps
```

See [FAQ](https://docs.ollama.com/faq) and [Troubleshooting](https://docs.ollama.com/troubleshooting) in the Ollama docs.
