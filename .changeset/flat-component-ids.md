---
default: major
---

# Component IDs are flat, and each component declares its capability in `provides`

A component is `components/<id>.yaml`, and its ID is the file name: `traefik`, not `ingress/traefik`. Its capability is `provides.capability`, such as `provides: {capability: ingress}`, and is no longer read from a folder. `index.json` drops the `capability` field of each component; read `provides.capability` instead. `requires` entries are `{component: <id>}` or `{capability: <capability>}` as before. Measurements move to `measurements/components/<id>.json`.

An installer reads a name in `enable` or `disable` as a component ID first, then as a capability. `catalog validate` fails when a capability is the ID of a component that offers another capability, and when a template has the ID of a component.

`indexVersion` is now 4. `testedOn` is empty for every component until the next full run measures the moved files again.
