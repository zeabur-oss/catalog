---
default: minor
---

# Zeabur Builder's AI builder can be turned on with component inputs

Zeabur Builder is now v0.3.1. The inputs `agentEnabled`, `agentProvider`, `agentModel`, `agentBaseUrl` and `agentAuth` set the chart's `agent` values, so the component can install the `agent` ClusterBuilder, which has a model write a Dockerfile for a repository without one. It stays off by default. `agentAuth` takes an API key Secret, Anthropic or OpenAI service account federation, or `tokenExchange` with another OAuth 2.0 token endpoint.

An input value can now be a map, which goes to its `path` as one object, so an installer can pass `agentAuth`. No component file has a map value yet.
