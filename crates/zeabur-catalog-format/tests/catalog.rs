//! Reads the catalog of this checkout strictly.

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;
    use std::path::Path;

    use zeabur_catalog_format::common::Profile;
    use zeabur_catalog_format::order::install_order;
    use zeabur_catalog_format::{Catalog, Contributions, values};

    fn catalog() -> Catalog {
        Catalog::read(&Path::new(env!("CARGO_MANIFEST_DIR")).join("../..")).unwrap()
    }

    #[test]
    fn every_component_and_template_reads() {
        let catalog = catalog();
        assert!(catalog.components.contains_key("cert-manager"));
        assert!(catalog.templates.values().all(|t| !t.manifests.is_empty()));
    }

    #[test]
    fn every_component_has_an_install_order_and_values() {
        let catalog = catalog();
        let components: &BTreeMap<_, _> = &catalog.components;
        for (id, component) in components {
            for profile in Profile::ALL {
                let Some(order) = install_order(components, id, profile).unwrap() else {
                    continue;
                };
                assert_eq!(order.last(), Some(id), "{id} on {profile}");
                let contributed = Contributions::before(components, &order, id).unwrap();
                for release in component.releases.iter().filter_map(|r| r.helm()) {
                    values::release_values(
                        component,
                        release,
                        profile,
                        contributed.to(id, &release.name),
                        &serde_json::Map::new(),
                    )
                    .unwrap_or_else(|e| panic!("{id}/{}: {e}", release.name));
                }
            }
        }
    }

    #[test]
    fn every_measurement_file_reads() {
        let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../measurements");
        for kind in ["components", "templates"] {
            for entry in std::fs::read_dir(root.join(kind)).unwrap() {
                let path = entry.unwrap().path();
                let text = std::fs::read_to_string(&path).unwrap();
                serde_json::from_str::<zeabur_catalog_format::api::MeasurementFile>(&text)
                    .unwrap_or_else(|e| panic!("{}: {e}", path.display()));
            }
        }
    }

    #[test]
    fn an_unknown_field_is_an_error() {
        let path =
            Path::new(env!("CARGO_MANIFEST_DIR")).join("../../components/traefik/component.yaml");
        let text = std::fs::read_to_string(path).unwrap() + "\nunknownField: 1\n";
        let read: Result<zeabur_catalog_format::Component, _> = serde_saphyr::from_str(&text);
        let error = read.unwrap_err().to_string();
        assert!(error.contains("unknownField"), "{error}");
    }
}
