package catalog

// Quantities is a component's own amount of resources: Kubernetes quantities
// by resource name, such as memory: 256Mi.
type Quantities map[string]string
