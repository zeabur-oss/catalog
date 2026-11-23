---
default: patch
---

# zeabur-catalog-engine writes resource quantities as Kubernetes does

The new `quantity` module gives the canonical text of a resource quantity, as `resource.Quantity` of apimachinery v0.37.2 writes it: `0.5` becomes `500m`, `1024Mi` becomes `1Gi`, and a value already in canonical form keeps its text. The API files will write the requests and limits of a rendered workload with it.
