//! Reads the catalog of this checkout strictly.

#[cfg(test)]
mod tests {
    use std::collections::BTreeMap;
    use std::path::Path;

    use zeabur_catalog_format::api::MeasurementFile;
    use zeabur_catalog_format::check::StrictProblem;
    use zeabur_catalog_format::common::{Distribution, Profile};
    use zeabur_catalog_format::order::install_order;
    use zeabur_catalog_format::problem::{FieldPath, ProblemKind};
    use zeabur_catalog_format::{Catalog, Component, Contributions, check, common, values};

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
    fn every_license_ref_has_a_url() {
        let catalog = catalog();
        let components = catalog
            .components
            .iter()
            .map(|(id, c)| (id, &c.metadata.license, &c.metadata.license_urls));
        let templates = catalog.templates.iter().map(|(id, t)| {
            (
                id,
                &t.template.metadata.license,
                &t.template.metadata.license_urls,
            )
        });
        for (id, license, urls) in components.chain(templates) {
            assert_eq!(common::license_url_problems(license, urls), [], "{id}");
        }
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
                check::from_yaml::<MeasurementFile>(&path, &text)
                    .unwrap_or_else(|e| panic!("{}: {e:?}", path.display()));
            }
        }
    }

    fn traefik() -> String {
        let path =
            Path::new(env!("CARGO_MANIFEST_DIR")).join("../../components/traefik/component.yaml");
        std::fs::read_to_string(path).unwrap()
    }

    /// Where strict reading finds each problem of `text`, and what.
    fn strict_problems(text: &str) -> Vec<(FieldPath, ProblemKind)> {
        check::from_yaml::<Component>(Path::new("component.yaml"), text)
            .unwrap_err()
            .into_iter()
            .map(|p| (p.at, p.kind))
            .collect()
    }

    fn found(at: FieldPath, kind: StrictProblem) -> (FieldPath, ProblemKind) {
        (at, kind.into())
    }

    #[test]
    fn the_check_names_an_unknown_field_by_its_path() {
        let text = traefik().replace("    condition: Ready", "    conditon: Ready");
        let at = FieldPath::root().key("health").index(0).key("conditon");
        assert!(strict_problems(&text).contains(&found(at, StrictProblem::UnknownField)));
    }

    #[test]
    fn the_check_sees_unknown_fields_inside_a_release() {
        let text = traefik().replace(
            "    chart: oci://",
            "    chartVersion: 1\n    chart: oci://",
        );
        let at = FieldPath::root()
            .key("releases")
            .index(1)
            .key("chartVersion");
        let problems = strict_problems(&text);
        assert!(
            problems.contains(&found(at, StrictProblem::UnknownField)),
            "{problems:?}"
        );
    }

    #[test]
    fn the_check_refuses_an_unknown_value() {
        let text = traefik().replace(
            "distributions: [k3s, rke2, kubesolo]",
            "distributions: [k3s, k0s]",
        );
        let at = FieldPath::root().key("distributions").index(1);
        assert!(strict_problems(&text).contains(&found(at, StrictProblem::UnknownValue)));
    }

    #[test]
    fn a_reader_ignores_what_it_does_not_know() {
        let text = traefik().replace(
            "distributions: [k3s, rke2, kubesolo]",
            "distributions: [k3s, k0s]",
        ) + "\nnewField: 1\n";
        let read: Component = serde_saphyr::from_str(&text).unwrap();
        assert_eq!(
            read.distributions,
            [Distribution::K3s, Distribution::Unknown]
        );
        assert!(read.known());

        let text = traefik().replace("type: helm", "type: kustomize");
        let read: Component = serde_saphyr::from_str(&text).unwrap();
        assert!(
            !read.known(),
            "a component with an unknown release type is not offered"
        );
    }
}
