---
default: patch
---

# A component that adds values to another needs a Helm release

`catalog check` now refuses `contributes` from a component without a `helm` release. Installers find the components they installed by their Helm releases, so they could not add the values of such a component again when they write the target. CrowdSec and VictoriaMetrics already have one.
