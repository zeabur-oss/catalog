//! Namespaces the engine never writes to.

/// Whether `namespace` is one Kubernetes keeps for itself, which is never adopted.
///
/// The `kube-` prefix "is reserved for Kubernetes system namespaces" (`kube-system`,
/// `kube-public`, `kube-node-lease`) [k8s-namespaces]: their objects are the cluster's own
/// components.
///
/// [k8s-namespaces]: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
#[must_use]
pub fn is_system_namespace(namespace: &str) -> bool {
    namespace.starts_with("kube-")
}

#[cfg(test)]
mod tests {
    use super::is_system_namespace;

    #[test]
    fn the_kube_prefix_is_kubernetes_own() {
        for name in ["kube-system", "kube-public", "kube-node-lease"] {
            assert!(is_system_namespace(name), "{name}");
        }
        for name in ["default", "shop", "kubeflow", "my-kube-app"] {
            assert!(!is_system_namespace(name), "{name}");
        }
    }
}
