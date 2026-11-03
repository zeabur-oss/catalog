---
default: minor
---

# System Upgrade Controller is no longer installed by default

`node-updates/system-upgrade-controller` is now optional on both profiles. Nothing installs a Plan for it, and zeabur-installer upgrades k3s and RKE2 itself: `apply` puts back the version it pins on a node that a Plan upgraded. Install it for Plans that leave the Kubernetes version alone, such as OS package upgrades.
