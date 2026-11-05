---
default: patch
---

# CrowdSec agents come back after their node restarts

A restarted agent Pod keeps its name, and the LAPI refused to register that name again, so the agent stayed down until someone ran `cscli machines delete`. A small `agent-recovery` Deployment in `crowdsec` now evicts such a Pod, and its replacement registers with a new name.
