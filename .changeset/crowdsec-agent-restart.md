---
default: patch
---

# CrowdSec agents come back after their node restarts

A restarted agent Pod keeps its name, and the LAPI refused to register that name again, so the agent stayed down until someone ran `cscli machines delete`. Agents now sign in with a client certificate from cert-manager, and the Traefik bouncer reaches the LAPI over HTTPS. CrowdSec now requires cert-manager (capability `certificates`). Its certificates last ten years, because agents don't reload a renewed certificate.
