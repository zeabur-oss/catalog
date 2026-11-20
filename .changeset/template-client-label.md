---
default: minor
---

# Templates reach each other's private services through a client label

A NetworkPolicy peer may now select the template's clients, Pods with the label `templates.zeabur.com/<id>-client: "true"`, as well as the template's own Pods. The OpenClaw Browser Sandbox lets in Pods with `templates.zeabur.com/openclaw-browser-sandbox-client: "true"` and drops the `clientApp` input. The OpenClaw template sets that label, so OpenClaw can now reach the sandbox. Ollama lets in any Pod with `templates.zeabur.com/ollama-client: "true"`, as its security notes say.
