//! The services of `template.yaml` match the rendered objects of the same name: a service's CPU
//! and memory are the sum of the object's containers, and its ports are their named ports. An
//! operator's custom resource gives its resources; the operator creates its ports.

use std::collections::BTreeMap;

use k8s_openapi::api::core::v1::{PodTemplateSpec, ResourceRequirements};
use serde::Deserialize;
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;
use zeabur_catalog_engine::pod_security;
use zeabur_catalog_engine::quantity::Quantity;
use zeabur_catalog_format::problem::LintRule;
use zeabur_catalog_format::template::{CpuMemory, Port, Template};

use super::Found;
use super::view::{Head, Loose, Obj, Spec, view};

/// The kinds that roll out their pods, whose pod template gives a service's containers.
const WORKLOADS: [&str; 3] = ["DaemonSet", "Deployment", "StatefulSet"];

/// The spec of a database object that gives the resources of its server.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Database {
    /// CloudNativePG Cluster, Dragonfly, MariaDB.
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
    /// Percona MongoDB: the first replica set.
    #[serde(default)]
    replsets: Loose<Vec<Loose<Resourced>>>,
    /// MOCO: the mysqld container.
    #[serde(default)]
    pod_template: Loose<MocoTemplate>,
}

#[derive(Debug, Default, Deserialize)]
struct Resourced {
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
}

#[derive(Debug, Default, Deserialize)]
struct MocoTemplate {
    #[serde(default)]
    spec: Loose<MocoPod>,
}

#[derive(Debug, Default, Deserialize)]
struct MocoPod {
    #[serde(default)]
    containers: Loose<Vec<Loose<MocoContainer>>>,
}

#[derive(Debug, Default, Deserialize)]
struct MocoContainer {
    #[serde(default)]
    name: Loose<String>,
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
}

/// Adds what the rendered objects `docs` give that the services of `template` do not say.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
pub(super) fn check(template: &Template, docs: &[Value], found: &mut Found) {
    for service in &template.services {
        let at = format!("service {}", service.name);
        let mut pod: Option<PodTemplateSpec> = None;
        let mut operand: Option<Obj<ResourceRequirements>> = None;
        let mut matched = false;
        for doc in docs {
            let head: Head = view(doc);
            if head.name() != service.name {
                continue;
            }
            let kind = head.kind.0.as_str();
            if WORKLOADS.contains(&kind) {
                match pod_security::pod_template(doc) {
                    Ok(template) => pod = template,
                    Err(e) => {
                        let (kind, message) = ("pod".to_owned(), e.to_string());
                        found.push(&at, LintRule::Invalid { kind, message });
                        continue;
                    }
                }
            } else {
                let spec = || view::<Spec<Loose<Database>>>(doc).spec.0;
                operand = match kind {
                    "Cluster" | "Dragonfly" | "MariaDB" => Some(spec().resources),
                    "PerconaServerMongoDB" => {
                        spec().replsets.0.into_iter().next().map(|r| r.0.resources)
                    }
                    "MySQLCluster" => spec()
                        .pod_template
                        .0
                        .spec
                        .0
                        .containers
                        .0
                        .into_iter()
                        .rfind(|c| c.0.name.0 == "mysqld")
                        .map(|c| c.0.resources)
                        .or(operand),
                    _ => continue,
                }
                .filter(|o| o.present);
            }
            matched = true;
        }
        if !matched {
            found.push(&at, LintRule::ServiceWithoutObject);
            continue;
        }
        let mut sums = Sums::default();
        if let Some(operand) = &operand {
            match &operand.typed {
                Ok(resources) => sums.add(resources, &at, found),
                Err(message) => {
                    let message = message.clone();
                    found.push(&at, LintRule::ServiceOperandResources { message });
                }
            }
        }
        let mut named = Vec::new();
        let containers = pod.and_then(|p| p.spec).map(|s| s.containers);
        for container in containers.iter().flatten() {
            if let Some(resources) = &container.resources {
                sums.add(resources, &at, found);
            }
            for port in container.ports.iter().flatten() {
                if !port.name.as_deref().unwrap_or_default().is_empty() {
                    named.push(Port {
                        name: port.name.clone().unwrap_or_default(),
                        port: u16::try_from(port.container_port).unwrap_or_default(),
                    });
                }
            }
        }
        let resources = &service.resources;
        sums.compare("requests", &resources.requests, &at, found);
        sums.compare("limits", &resources.limits, &at, found);
        if operand.is_none() {
            let mut declared = service.ports.clone();
            declared.sort_by_key(|p| p.port);
            named.sort_by_key(|p| p.port);
            if declared != named {
                found.push(&at, LintRule::ServicePorts { declared, named });
            }
        }
    }
}

/// The CPU and memory requests and limits of the containers, summed.
#[derive(Debug, Default)]
struct Sums(BTreeMap<(&'static str, &'static str), Quantity>);

impl Sums {
    fn add(&mut self, resources: &ResourceRequirements, at: &str, found: &mut Found) {
        let levels = [
            ("requests", resources.requests.as_ref()),
            ("limits", resources.limits.as_ref()),
        ];
        for (level, list) in levels {
            for resource in ["cpu", "memory"] {
                let Some(text) = list.and_then(|l| l.get(resource)) else {
                    continue;
                };
                let sum = Quantity::parse(&text.0).and_then(|q| {
                    self.0
                        .get(&(level, resource))
                        .map_or(Some(q), |sum| sum.checked_add(q))
                });
                if let Some(sum) = sum {
                    self.0.insert((level, resource), sum);
                } else {
                    let value = text.0.clone();
                    let rule = LintRule::ServiceQuantity {
                        level,
                        resource,
                        value,
                    };
                    found.push(at, rule);
                }
            }
        }
    }

    /// Adds a problem for each of `declared` that is not the sum.
    fn compare(&self, level: &'static str, declared: &CpuMemory, at: &str, found: &mut Found) {
        for (resource, text) in [("cpu", &declared.cpu), ("memory", &declared.memory)] {
            let Some(want) = Quantity::parse(text) else {
                let value = text.clone();
                found.push(
                    at,
                    LintRule::ServiceQuantity {
                        level,
                        resource,
                        value,
                    },
                );
                continue;
            };
            let sum = self.0.get(&(level, resource));
            if sum != Some(&want) {
                found.push(
                    at,
                    LintRule::ServiceResources {
                        level,
                        resource,
                        declared: text.clone(),
                        rendered: sum.map(Quantity::to_string),
                    },
                );
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use zeabur_catalog_format::problem::LintRule;
    use zeabur_catalog_format::template::Template;

    use super::super::Found;
    use super::super::tests::{SAFE, assert_breaks, docs, replace, service, template};

    /// The rules the objects of `manifest` break of the service rules.
    fn services(template: &Template, manifest: &str) -> Vec<LintRule> {
        let mut found = Found::default();
        super::check(template, &docs(manifest), &mut found);
        found.0.into_iter().map(|p| p.rule).collect()
    }

    /// The template with the service `name`, which requests `requests` and limits `limits`, as
    /// `(cpu, memory)`, and has `ports`.
    fn declared(name: &str, requests: (&str, &str), limits: (&str, &str), ports: &str) -> Template {
        let mut template = template(&[], &[]);
        let mut service = service(name, "restricted");
        service.resources.requests.cpu = requests.0.to_owned();
        service.resources.requests.memory = requests.1.to_owned();
        service.resources.limits.cpu = limits.0.to_owned();
        service.resources.limits.memory = limits.1.to_owned();
        service.ports = serde_saphyr::from_str(ports).unwrap();
        template.services = vec![service];
        template
    }

    #[test]
    fn a_service_is_the_sum_of_its_containers() {
        let sidecar = replace(
            SAFE,
            "      containers:\n",
            "      containers:
        - name: sidecar
          ports: [{name: http, containerPort: 8080}]
          resources:
            requests: {cpu: 5m, memory: 8Mi}
            limits: {cpu: 1, memory: 32Mi}
",
        );
        let app = |cpu, ports| declared("app", ("15m", "24Mi"), (cpu, "64Mi"), ports);
        let http = "[{name: http, port: 8080}]";
        assert_eq!(services(&app("1100m", http), &sidecar), []);
        let rules = services(&app("1", http), &sidecar);
        assert_breaks!(
            rules,
            LintRule::ServiceResources {
                level: "limits",
                resource: "cpu",
                ..
            }
        );
        assert_breaks!(
            services(&app("1100m", "[]"), &sidecar),
            LintRule::ServicePorts { .. }
        );
        let web = declared("web", ("15m", "24Mi"), ("1100m", "64Mi"), "[]");
        assert_breaks!(services(&web, &sidecar), LintRule::ServiceWithoutObject);
    }

    #[test]
    fn a_mysql_service_is_its_mysqld_container() {
        let manifest = "apiVersion: moco.cybozu.com/v1beta2
kind: MySQLCluster
metadata: {name: db}
spec:
  podTemplate:
    spec:
      containers:
        - name: mysqld
          resources:
            requests: {cpu: 100m, memory: 256Mi}
            limits: {cpu: \"1\", memory: 1Gi}
";
        let db = |memory| declared("db", ("100m", "256Mi"), ("1", memory), "[]");
        assert_eq!(services(&db("1Gi"), manifest), []);
        let rules = services(&db("2Gi"), manifest);
        assert_breaks!(
            rules,
            LintRule::ServiceResources {
                level: "limits",
                resource: "memory",
                ..
            }
        );
    }
}
