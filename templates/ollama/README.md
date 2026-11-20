## Introduction

[Ollama](https://github.com/ollama/ollama) runs large language models and serves them through an HTTP API.

This template runs one Ollama on the CPU, with a 50 GiB volume for models. It has no domain, no web page and no sign-in. Only Pods you allow, in the same namespace, can reach its API. No model is preloaded.

## Getting started

1. Other workloads reach Ollama at Service `ollama`, port `11434`, for example `http://ollama:11434`. The template allows only Pods in the same namespace that carry both of these labels:

   ```yaml
   templates.zeabur.com/template: ollama
   templates.zeabur.com/ollama-client: "true"
   ```

2. To use the API from your own computer, forward the port, then use `http://localhost:11434`:

   ```sh
   kubectl -n <namespace> port-forward service/ollama 11434
   ```

3. Pull a model. Read the model's license and check its size first. The Pod has an 8 GiB memory limit, so large models do not fit:

   ```sh
   kubectl -n <namespace> exec deployment/ollama -- ollama pull <model>
   ```

There is no password, so the template creates no Secret.

## Configuration

### Connect an OpenAI client

Ollama has an OpenAI-compatible API. Set the client's base URL to `http://ollama:11434/v1`. The client needs an API key value, but Ollama ignores it, so use any text. See [OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility) in the Ollama docs.

### List and remove models

```sh
kubectl -n <namespace> exec deployment/ollama -- ollama ls
kubectl -n <namespace> exec deployment/ollama -- ollama rm <model>
```

See [CLI reference](https://docs.ollama.com/cli) in the Ollama docs.

## Troubleshooting

### A connection from a Pod times out

The Pod needs both labels from Getting started. Check the labels of the Pod that fails, and add the missing one to the Pod template of that workload.

### A model is slow or does not load

This template runs on the CPU, with an 8 GiB memory limit and one loaded model at a time. Run `ollama ps` to see what is loaded, and choose a smaller model if the model does not fit. Read the log for the reason:

```sh
kubectl -n <namespace> exec deployment/ollama -- ollama ps
kubectl -n <namespace> logs deployment/ollama
```

See [FAQ](https://docs.ollama.com/faq) and [Troubleshooting](https://docs.ollama.com/troubleshooting) in the Ollama docs.
