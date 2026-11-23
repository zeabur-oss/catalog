//! Renders the cases of `conformance/template/` and every template of the repository.

#[cfg(test)]
mod tests {
    #![expect(
        clippy::disallowed_types,
        clippy::disallowed_methods,
        reason = "the cases and the rendered objects are JSON of any shape"
    )]

    use std::collections::BTreeMap;
    use std::path::{Path, PathBuf};

    use serde_json::{Value, json};
    use zeabur_catalog_engine::template::{self, Options, RenderError};
    use zeabur_catalog_format::{Catalog, TemplateDir};

    fn root() -> PathBuf {
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../..")
    }

    fn catalog() -> Catalog {
        Catalog::read(&root()).unwrap_or_else(|e| panic!("{e}"))
    }

    /// `file`, a template with only the fields a case reads, with the others a template file
    /// must have.
    fn complete(mut file: Value) -> zeabur_catalog_format::Template {
        fill(
            &mut file,
            &json!({
                "apiVersion": "catalog.zeabur.com/v1",
                "kind": "Template",
                "metadata": {
                    "name": "A",
                    "version": "1",
                    "summary": "s",
                    "locales": {},
                    "license": "MIT",
                    "homepage": "https://example.com",
                    "docs": "https://example.com",
                },
                "services": [],
                "security": {"summary": "s", "risks": []},
                "smoke": [],
            }),
        );
        let quantities = json!({"cpu": "10m", "memory": "16Mi"});
        let service = json!({
            "podSecurity": "restricted",
            "resources": {"requests": quantities, "limits": quantities},
        });
        for service_file in file["services"].as_array_mut().into_iter().flatten() {
            fill(service_file, &service);
        }
        // `get_mut`, not `file["inputs"]`, which would add `inputs: null` to a case without it.
        for input in file
            .get_mut("inputs")
            .and_then(Value::as_array_mut)
            .into_iter()
            .flatten()
        {
            fill(input, &json!({"description": "d"}));
        }
        serde_json::from_value(file).unwrap()
    }

    /// Adds each key of `defaults` that `value` lacks, in nested objects too.
    fn fill(value: &mut Value, defaults: &Value) {
        let (Some(value), Some(defaults)) = (value.as_object_mut(), defaults.as_object()) else {
            return;
        };
        for (key, default) in defaults {
            match value.get_mut(key) {
                Some(existing) => fill(existing, default),
                None => {
                    value.insert(key.clone(), default.clone());
                }
            }
        }
    }

    /// The `want.error` of a case.
    fn case_error(error: &RenderError) -> Value {
        assert!(!error.is_internal(), "{error}");
        error.input().map_or_else(
            || json!({"code": error.code()}),
            |input| json!({"code": error.code(), "input": input}),
        )
    }

    #[test]
    fn renders_as_the_conformance_cases_want() {
        let dir = root().join("conformance/template");
        let mut ran = 0;
        for entry in std::fs::read_dir(&dir).unwrap() {
            let path = entry.unwrap().path();
            let case: Value =
                serde_saphyr::from_str(&std::fs::read_to_string(&path).unwrap()).unwrap();
            // The other cases are for a client's API: requirements and words.
            let Some(manifests) = case["manifests"].as_str() else {
                continue;
            };
            let id = case["id"].as_str().unwrap();
            let template_dir = TemplateDir {
                template: complete(case["template"].clone()),
                manifests: manifests.to_owned(),
            };
            let options = &case["options"];
            let inputs: BTreeMap<String, String> =
                serde_json::from_value(options["inputs"].clone()).unwrap_or_default();
            let got = template::render(
                id,
                &template_dir,
                &Options {
                    namespace: options["namespace"].as_str().unwrap(),
                    domain: options["domain"].as_str(),
                    cluster_issuer: options["clusterIssuer"].as_str(),
                    inputs: &inputs,
                },
            );
            let got = match got {
                Ok(objects) => json!({"objects": objects}),
                Err(error) => json!({"error": case_error(&error)}),
            };
            assert_eq!(got, case["want"], "{}", path.display());
            ran += 1;
        }
        assert!(ran > 0, "no render cases in {}", dir.display());
    }

    /// A value for each input that a template needs and gives no default.
    const INPUTS: &[(&str, &str)] = &[
        ("adminDomain", "admin.example.com"),
        ("adminEmail", "admin@example.com"),
        ("llmBaseUrl", "https://llm.example.com/v1"),
        ("llmModel", "model"),
        ("model", "model"),
        ("ownerEmail", "owner@example.com"),
        ("providerSecret", "provider"),
        ("smtpFrom", "ghost@example.com"),
        ("smtpHost", "smtp.example.com"),
        ("smtpPort", "587"),
        ("smtpSecret", "smtp"),
        ("storageBucket", "bucket"),
        ("storageEndpoint", "https://s3.example.com"),
        ("storageRegion", "us-east-1"),
        ("storageSecret", "s3"),
    ];

    fn inputs(dir: &TemplateDir) -> BTreeMap<String, String> {
        dir.template
            .inputs
            .iter()
            .filter(|i| i.required && i.default.is_none() && i.generate.is_none())
            .map(|i| {
                let value = INPUTS
                    .iter()
                    .find(|(name, _)| *name == i.name)
                    .unwrap_or_else(|| panic!("no test value for the input {}", i.name))
                    .1;
                (i.name.clone(), value.to_owned())
            })
            .collect()
    }

    #[test]
    fn every_template_renders_into_its_namespace_with_its_labels_and_keeps_its_traffic_to_itself() {
        for (id, dir) in &catalog().templates {
            let inputs = inputs(dir);
            let options = Options {
                namespace: "shop",
                domain: Some("app.example.com"),
                cluster_issuer: Some("letsencrypt"),
                inputs: &inputs,
            };
            let objects =
                template::render(id, dir, &options).unwrap_or_else(|e| panic!("{id}: {e}"));
            assert!(!objects.is_empty(), "{id}");
            for object in &objects {
                assert_eq!(
                    object.pointer("/metadata/namespace"),
                    Some(&Value::from("shop")),
                    "{id}: {object}"
                );
                for (key, value) in template::labels(id, &dir.template) {
                    assert_eq!(
                        object.pointer("/metadata/labels").and_then(|l| l.get(&key)),
                        Some(&Value::from(value)),
                        "{id}: {object}"
                    );
                }
                // Every NetworkPolicy selects the template's own pods, so other services of the
                // namespace keep their traffic.
                if object["kind"] == "NetworkPolicy" {
                    assert_eq!(
                        object.pointer(
                            "/spec/podSelector/matchLabels/templates.zeabur.com~1template"
                        ),
                        Some(&Value::from(id.as_str())),
                        "{id}: {object}"
                    );
                }
            }
        }
    }

    #[test]
    fn refuses_a_domain_that_ends_the_yaml_string() {
        let catalog = catalog();
        let miniflux = &catalog.templates["miniflux"];
        let none = BTreeMap::new();
        let options = Options {
            namespace: "shop",
            domain: Some("rss.example.com"),
            cluster_issuer: Some("letsencrypt"),
            inputs: &none,
        };
        assert!(template::render("miniflux", miniflux, &options).is_ok());
        let escaping = Options {
            domain: Some("a.example.com'\nkind: Secret"),
            ..options
        };
        assert_eq!(
            template::render("miniflux", miniflux, &escaping).map_err(|e| e.code()),
            Err("invalid-domain")
        );
    }

    #[test]
    fn a_template_without_a_domain_needs_none() {
        let catalog = catalog();
        let pages = &catalog.templates["error-pages"];
        assert!(!template::routes_domain(&pages.template));
        let none = BTreeMap::new();
        let options = Options {
            namespace: "shop",
            domain: None,
            cluster_issuer: None,
            inputs: &none,
        };
        let objects = template::render("error-pages", pages, &options).unwrap();
        assert!(objects.iter().any(|o| o["kind"] == "Deployment"));
    }

    #[test]
    fn refuses_duplicate_input_names_and_secret_keys() {
        let dir = |inputs: [(&str, &str, &str); 2]| {
            let inputs: Vec<Value> = inputs
                .iter()
                .map(|(name, secret, key)| {
                    json!({
                        "name": name,
                        "generate": {"type": "random", "bytes": 32, "encoding": "base64url"},
                        "secret": {"name": secret, "key": key},
                        "validation": {"pattern": "x"},
                    })
                })
                .collect();
            TemplateDir {
                template: complete(json!({ "inputs": inputs })),
                manifests: String::new(),
            }
        };
        assert!(template::check("t", &dir([("a", "s", "k"), ("b", "s", "l")])).is_ok());
        assert!(template::check("t", &dir([("a", "s", "k"), ("a", "s", "l")])).is_err());
        assert!(template::check("t", &dir([("a", "s", "k"), ("b", "s", "k")])).is_err());
    }
}
