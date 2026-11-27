//! How far a step of an install, or a health check, has come.
//!
//! Readiness follows kstatus [kstatus] through [`crate::readiness`]. A `HelmChart` has no
//! `observedGeneration`, and helm-controller replaces its Job whenever the chart changes, so a
//! chart is ready when the Job its status names was created at or after the engine's last write
//! of the chart and has completed [helm-controller].
//!
//! [kstatus]: https://github.com/kubernetes-sigs/cli-utils/blob/v0.37.2/pkg/kstatus/README.md
//! [helm-controller]: https://github.com/k3s-io/helm-controller/blob/v0.16.27/pkg/controllers/chart/chart.go

use std::collections::BTreeMap;

use k8s_openapi::api::batch::v1::Job;
use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use k8s_openapi::jiff::Timestamp;
use kube_core::DynamicObject;
use serde::Deserialize;
use zeabur_catalog_format::component::{Health, HealthKind};

use crate::objects::{FIELD_MANAGER, HelmChart, ServedResource};
use crate::readiness::{self, Explained, GenericStatus, Reason, Status};

/// How far one step, or one check, has come.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Progress {
    /// Not written yet.
    Pending,
    /// Written; the controller works on it.
    Waiting,
    /// Done.
    Ready,
    /// It will not get ready without a change: why.
    Failed(String),
}

/// When the engine last wrote the object.
fn written_at(metadata: &ObjectMeta) -> Option<Timestamp> {
    metadata
        .managed_fields
        .iter()
        .flatten()
        .filter(|m| m.manager.as_deref() == Some(FIELD_MANAGER))
        .filter_map(|m| m.time.as_ref().map(|t| t.0))
        .max()
}

/// Whether a `HelmChart` is installed as last written.
///
/// `job`, the Job its status names, must be created at or after the engine's last write and
/// complete, and the `Failed` condition must not be `True`. The condition has no time, so a failure
/// counts only while its Job is not older than the write.
#[must_use]
pub fn chart_progress(chart: &HelmChart, job: Option<&Job>) -> Progress {
    progress_since(written_at(&chart.metadata), chart, job)
}

/// Whether the `HelmChart` a `HelmChartConfig` changes is installed with it: as
/// [`chart_progress`], with the Job created at or after the engine's last write of the config.
#[must_use]
pub fn config_progress(config: &ObjectMeta, chart: &HelmChart, job: Option<&Job>) -> Progress {
    progress_since(written_at(config), chart, job)
}

fn progress_since(written: Option<Timestamp>, chart: &HelmChart, job: Option<&Job>) -> Progress {
    let conditions = chart
        .status
        .as_ref()
        .map_or(&[][..], |s| s.conditions.as_slice());
    let current = job.map(|job| {
        let created = job.metadata.creation_timestamp.as_ref().map(|t| t.0);
        match (written, created) {
            (Some(written), Some(created)) => created >= truncate(written),
            (None, Some(_)) => true,
            (_, None) => false,
        }
    });
    if current != Some(false)
        && let Some(failed) = conditions
            .iter()
            .find(|c| c.kind == "Failed" && c.status == "True")
    {
        return Progress::Failed(failed.message.clone().unwrap_or_default());
    }
    let Some(job) = job else {
        return Progress::Waiting;
    };
    if current != Some(true) {
        return Progress::Waiting;
    }
    let state = readiness::job(job);
    match (state.status, state.reason) {
        (Status::Current, Reason::JobComplete) => Progress::Ready,
        (Status::Failed, _) => Progress::Failed(format!(
            "the Helm Job {} failed",
            job.metadata.name.as_deref().unwrap_or_default()
        )),
        _ => Progress::Waiting,
    }
}

/// Kubernetes keeps timestamps to the second, so a Job created in the second of the write
/// counts as after it.
fn truncate(t: Timestamp) -> Timestamp {
    Timestamp::from_second(t.as_second()).unwrap_or(t)
}

/// Whether an inline object of `group` and `kind` is ready, by kstatus.
///
/// The built-in kinds that [`crate::readiness`] knows follow the rule of their kind, a CRD
/// follows `Established`, and the rest follow the generic rule. A failure names the object and
/// its reason or condition.
#[must_use]
pub fn object_progress(
    group: &str,
    kind: &str,
    object: Option<&DynamicObject>,
    now: Timestamp,
) -> Progress {
    let Some(object) = object else {
        return Progress::Pending;
    };
    kstatus(group, kind, object, now).unwrap_or(Progress::Waiting)
}

/// The kinds with a rule of their own in [`crate::readiness`], as `(group, kind)`.
const OWN_RULE: &[(&str, &str)] = &[
    ("apps", "Deployment"),
    ("apps", "StatefulSet"),
    ("apps", "DaemonSet"),
    ("", "Pod"),
    ("batch", "Job"),
    ("apiextensions.k8s.io", "CustomResourceDefinition"),
];

/// The kstatus answer for `object`; `None` when it does not parse as its kind.
fn kstatus(group: &str, kind: &str, object: &DynamicObject, now: Timestamp) -> Option<Progress> {
    let name = object.metadata.name.as_deref().unwrap_or_default();
    let answer = |explained: Explained<'_>| progress(kind, name, explained);
    Some(match (group, kind) {
        ("apps", "Deployment") => answer(readiness::deployment(&parsed(object)?).into()),
        ("apps", "StatefulSet") => answer(readiness::stateful_set(&parsed(object)?).into()),
        ("apps", "DaemonSet") => answer(readiness::daemon_set(&parsed(object)?).into()),
        ("", "Pod") => answer(readiness::pod(&parsed(object)?, now).into()),
        ("batch", "Job") => answer(readiness::job(&parsed(object)?).into()),
        ("apiextensions.k8s.io", "CustomResourceDefinition") => {
            answer(readiness::custom_resource_definition(&parsed(object)?))
        }
        _ => answer(readiness::generic(&object.metadata, &status_of(object)?)),
    })
}

fn progress(kind: &str, name: &str, explained: Explained<'_>) -> Progress {
    match explained.readiness.status {
        Status::Current => Progress::Ready,
        Status::InProgress => Progress::Waiting,
        Status::Failed => Progress::Failed(explained.cause.map_or_else(
            || format!("{kind} {name}: {}", explained.readiness.reason),
            |cause| format!("{kind} {name}: {cause}"),
        )),
    }
}

fn parsed<K: serde::de::DeserializeOwned>(object: &DynamicObject) -> Option<K> {
    serde_json::to_value(object)
        .and_then(serde_json::from_value)
        .ok()
}

/// The `status` fields the generic rule reads.
fn status_of(object: &DynamicObject) -> Option<GenericStatus> {
    #[derive(Deserialize)]
    struct Body {
        #[serde(default)]
        status: GenericStatus,
    }
    serde_json::from_value::<Body>(object.data.clone())
        .ok()
        .map(|body| body.status)
}

/// The served resource of a health check's kind: the one named by its `plural.group`.
#[must_use]
pub fn health_resource(kind: HealthKind, served: &[ServedResource]) -> Option<&ServedResource> {
    let resource = kind.resource();
    let (plural, group) = resource.split_once('.').unwrap_or((resource, ""));
    served
        .iter()
        .find(|r| r.resource == plural && r.group == group)
}

/// Whether a health check passes on `objects` of `resource`, the objects it names.
///
/// Each object of a kind with a rule of its own is Current by kstatus. Each other object passes
/// the generic rule's checks of deletion, generation, `Reconciling` and `Stalled`, and has the
/// check's condition `True`. No object yet is waiting. A failure names the object and its reason
/// or condition.
#[must_use]
pub fn check_progress(
    check: &Health,
    resource: &ServedResource,
    objects: &[DynamicObject],
    now: Timestamp,
) -> Progress {
    if objects.is_empty() {
        return Progress::Waiting;
    }
    let (group, kind) = (resource.group.as_str(), resource.kind.as_str());
    let own_rule = OWN_RULE.contains(&(group, kind));
    let mut waiting = false;
    for object in objects {
        let progress = if own_rule {
            kstatus(group, kind, object, now).unwrap_or(Progress::Waiting)
        } else {
            condition_progress(kind, object, check.condition.as_deref().unwrap_or_default())
        };
        match progress {
            Progress::Ready => {}
            Progress::Failed(why) => return Progress::Failed(why),
            Progress::Pending | Progress::Waiting => waiting = true,
        }
    }
    if waiting {
        Progress::Waiting
    } else {
        Progress::Ready
    }
}

/// `condition` of `object` is `True`, once the generic rule finds the object observed, not
/// reconciling and not stalled.
fn condition_progress(kind: &str, object: &DynamicObject, condition: &str) -> Progress {
    let Some(status) = status_of(object) else {
        return Progress::Waiting;
    };
    let explained = readiness::generic(&object.metadata, &status);
    let reason = explained.readiness.reason;
    if explained.readiness.status == Status::Failed {
        let name = object.metadata.name.as_deref().unwrap_or_default();
        return progress(kind, name, explained);
    }
    let unsettled = [
        Reason::Terminating,
        Reason::LatestGenerationNotObserved,
        Reason::Reconciling,
    ];
    let holds = status
        .conditions
        .iter()
        .any(|c| c.type_ == condition && c.status == "True");
    if holds && !unsettled.contains(&reason) {
        Progress::Ready
    } else {
        Progress::Waiting
    }
}

/// A check's `timeout`, such as `60s` or `5m`, in seconds; 300 when it does not read.
#[must_use]
pub fn timeout_seconds(text: &str) -> i64 {
    let (number, unit) = text.split_at(
        text.find(|c: char| !c.is_ascii_digit())
            .unwrap_or(text.len()),
    );
    let n: i64 = number.parse().unwrap_or(0);
    match unit {
        "s" | "" if n > 0 => n,
        "m" if n > 0 => n * 60,
        "h" if n > 0 => n * 3600,
        _ => 300,
    }
}

/// The selector of a check as `key=value,…`, for a label-selected list.
#[must_use]
pub fn selector(labels: &BTreeMap<String, String>) -> String {
    labels
        .iter()
        .map(|(k, v)| format!("{k}={v}"))
        .collect::<Vec<_>>()
        .join(",")
}

#[cfg(test)]
mod tests {
    #![expect(
        clippy::disallowed_types,
        reason = "the cases are objects as JSON, the shape the API server gives"
    )]

    use serde_json::{Value, json};
    use zeabur_catalog_format::component::HealthResource;

    use super::*;

    /// The health check `controller` of cert-manager.
    fn controller() -> Health {
        Health {
            name: "controller".to_owned(),
            resource: HealthResource {
                kind: HealthKind::Deployment,
                namespace: Some("cert-manager".to_owned()),
                name: Some("cert-manager".to_owned()),
                selector: None,
                all: false,
            },
            condition: None,
            timeout: "60s".to_owned(),
            advice: String::new(),
        }
    }

    fn chart(written: &str, conditions: &Value) -> HelmChart {
        serde_json::from_value(json!({
            "metadata": {
                "name": "cert-manager",
                "managedFields": [
                    {"manager": "zeabur", "operation": "Apply", "time": written},
                    {"manager": "helm-controller", "operation": "Update", "time": "2030-01-01T00:00:00Z"},
                ],
            },
            "status": {"jobName": "helm-install-cert-manager", "conditions": conditions},
        }))
        .unwrap()
    }

    fn job(created: &str, conditions: &Value) -> Job {
        serde_json::from_value(json!({
            "metadata": {"name": "helm-install-cert-manager", "creationTimestamp": created},
            "status": {"conditions": conditions},
        }))
        .unwrap()
    }

    fn complete() -> Value {
        json!([
            {"type": "SuccessCriteriaMet", "status": "True"},
            {"type": "Complete", "status": "True"},
        ])
    }

    #[test]
    fn a_chart_is_ready_when_the_job_after_the_last_write_completed() {
        let written = "2026-11-19T10:00:00.400Z";
        let c = chart(written, &json!([]));
        assert_eq!(chart_progress(&c, None), Progress::Waiting);
        // The Job of the write before.
        let old = job("2026-11-19T09:00:00Z", &complete());
        assert_eq!(chart_progress(&c, Some(&old)), Progress::Waiting);
        // Created in the same second as the write.
        let now = job("2026-11-19T10:00:00Z", &complete());
        assert_eq!(chart_progress(&c, Some(&now)), Progress::Ready);
        let running = job("2026-11-19T10:00:01Z", &json!([]));
        assert_eq!(chart_progress(&c, Some(&running)), Progress::Waiting);
    }

    #[test]
    fn a_failed_chart_or_job_is_a_failure() {
        let failed = chart(
            "2026-11-19T10:00:00Z",
            &json!([{"type": "Failed", "status": "True", "message": "chart not found"}]),
        );
        assert_eq!(
            chart_progress(&failed, None),
            Progress::Failed("chart not found".to_owned())
        );

        let c = chart("2026-11-19T10:00:00Z", &json!([]));
        let j = job(
            "2026-11-19T10:00:02Z",
            &json!([{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded"}]),
        );
        assert!(matches!(chart_progress(&c, Some(&j)), Progress::Failed(_)));
    }

    #[test]
    fn a_failure_of_the_job_before_the_last_write_waits_for_the_next_job() {
        let failed = chart(
            "2026-11-19T10:00:00Z",
            &json!([{"type": "Failed", "status": "True", "message": "the old values"}]),
        );
        let before = job(
            "2026-11-19T09:59:00Z",
            &json!([{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded"}]),
        );
        assert_eq!(chart_progress(&failed, Some(&before)), Progress::Waiting);
        let after = job(
            "2026-11-19T10:00:01Z",
            &json!([{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded"}]),
        );
        assert!(matches!(
            chart_progress(&failed, Some(&after)),
            Progress::Failed(_)
        ));
    }

    fn object(v: Value) -> DynamicObject {
        serde_json::from_value(v).unwrap()
    }

    #[test]
    fn an_inline_object_is_ready_by_the_kstatus_rule_of_its_kind() {
        let now = Timestamp::from_second(2_000_000_000).unwrap();
        let progress = |group, kind, v: Value| object_progress(group, kind, Some(&object(v)), now);
        assert_eq!(
            object_progress("", "ConfigMap", None, now),
            Progress::Pending
        );
        let plain = json!({"kind": "ConfigMap", "apiVersion": "v1", "metadata": {}});
        let issuer = |status: &str| {
            json!({"metadata": {"name": "letsencrypt", "generation": 1},
                "status": {"conditions": [{"type": "Ready", "status": status}]}})
        };
        let unobserved = json!({"metadata": {"name": "letsencrypt", "generation": 2},
            "status": {"observedGeneration": 1,
                "conditions": [{"type": "Ready", "status": "True"}]}});
        let crd = |conditions: Value| {
            json!({"metadata": {"name": "issuers.cert-manager.io"},
                "spec": {"group": "cert-manager.io", "scope": "Namespaced",
                    "names": {"plural": "issuers", "kind": "Issuer"}, "versions": []},
                "status": {"conditions": conditions}})
        };
        let fresh_deployment = json!({"metadata": {"name": "webhook", "generation": 1},
            "spec": {"replicas": 1, "selector": {}, "template": {}}});
        let cases = [
            ("a ConfigMap", "", "ConfigMap", plain, Progress::Ready),
            (
                "not ready",
                "cert-manager.io",
                "Issuer",
                issuer("False"),
                Progress::Waiting,
            ),
            (
                "ready",
                "cert-manager.io",
                "Issuer",
                issuer("True"),
                Progress::Ready,
            ),
            (
                "an older generation",
                "cert-manager.io",
                "Issuer",
                unobserved,
                Progress::Waiting,
            ),
            (
                "a CRD not established",
                "apiextensions.k8s.io",
                "CustomResourceDefinition",
                crd(json!([])),
                Progress::Waiting,
            ),
            (
                "an established CRD",
                "apiextensions.k8s.io",
                "CustomResourceDefinition",
                crd(json!([{"type": "Established", "status": "True"}])),
                Progress::Ready,
            ),
            (
                "a Deployment with no replica yet",
                "apps",
                "Deployment",
                fresh_deployment,
                Progress::Waiting,
            ),
        ];
        for (name, group, kind, v, expected) in cases {
            assert_eq!(progress(group, kind, v), expected, "{name}");
        }
    }

    #[test]
    fn a_failed_object_says_its_reason_and_condition() {
        let now = Timestamp::from_second(2_000_000_000).unwrap();
        let stalled = object(
            json!({"metadata": {"name": "pg"}, "status": {"conditions": [
            {"type": "Stalled", "status": "True", "reason": "BadSpec",
             "message": "no such storage class"}]}}),
        );
        assert_eq!(
            object_progress("postgresql.cnpg.io", "Cluster", Some(&stalled), now),
            Progress::Failed("Cluster pg: Stalled: BadSpec: no such storage class".to_owned())
        );
        let past_deadline = object(json!({"metadata": {"name": "webhook"},
            "spec": {"selector": {}, "template": {}, "progressDeadlineSeconds": 600},
            "status": {"conditions": [{"type": "Progressing", "status": "False",
                "reason": "ProgressDeadlineExceeded"}]}}));
        assert_eq!(
            object_progress("apps", "Deployment", Some(&past_deadline), now),
            Progress::Failed("Deployment webhook: ProgressDeadlineExceeded".to_owned())
        );
    }

    fn served(group: &str, kind: &str) -> ServedResource {
        let lower = kind.to_ascii_lowercase();
        ServedResource {
            group: group.to_owned(),
            version: "v1".to_owned(),
            resource: if lower.ends_with('s') {
                format!("{lower}es")
            } else {
                format!("{lower}s")
            },
            kind: kind.to_owned(),
            namespaced: true,
            verbs: Vec::new(),
        }
    }

    #[test]
    fn a_health_check_kind_is_found_in_discovery() {
        let all = [
            served("", "Node"),
            served("", "Pod"),
            served("apps", "Deployment"),
            served("longhorn.io", "Node"),
            served("gateway.networking.k8s.io", "GatewayClass"),
        ];
        let found = |kind| health_resource(kind, &all).map(|r| r.group.as_str());
        assert_eq!(found(HealthKind::Deployment), Some("apps"));
        assert_eq!(found(HealthKind::Pod), Some(""));
        assert_eq!(found(HealthKind::LonghornNode), Some("longhorn.io"));
        assert_eq!(
            found(HealthKind::GatewayClass),
            Some("gateway.networking.k8s.io")
        );
        assert_eq!(found(HealthKind::ApiService), None);
    }

    #[test]
    fn a_health_check_reads_its_workloads_by_kstatus() {
        let check = &controller();
        let resource = served("apps", "Deployment");
        let now = Timestamp::from_second(2_000_000_000).unwrap();
        assert_eq!(
            check_progress(check, &resource, &[], now),
            Progress::Waiting
        );

        let deployment = |available: i32| {
            object(json!({
                "apiVersion": "apps/v1",
                "kind": "Deployment",
                "metadata": {"name": "cert-manager", "generation": 1},
                "spec": {"replicas": 1, "selector": {}, "template": {}},
                "status": {
                    "observedGeneration": 1,
                    "replicas": 1,
                    "updatedReplicas": 1,
                    "readyReplicas": available,
                    "availableReplicas": available,
                    "conditions": [{"type": "Available", "status": if available > 0 { "True" } else { "False" }}],
                },
            }))
        };
        assert_eq!(
            check_progress(check, &resource, &[deployment(0)], now),
            Progress::Waiting
        );
        assert_eq!(
            check_progress(check, &resource, &[deployment(1)], now),
            Progress::Ready
        );
    }

    #[test]
    fn a_health_check_condition_counts_once_the_generation_is_observed() {
        let check = Health {
            name: "nodes-ready".to_owned(),
            resource: HealthResource {
                kind: HealthKind::LonghornNode,
                namespace: Some("longhorn-system".to_owned()),
                name: None,
                selector: None,
                all: true,
            },
            condition: Some("Ready".to_owned()),
            timeout: "60s".to_owned(),
            advice: String::new(),
        };
        let resource = served("longhorn.io", "Node");
        let now = Timestamp::from_second(2_000_000_000).unwrap();
        let node = |observed: i64| {
            object(json!({"metadata": {"name": "n1", "generation": 2},
                "status": {"observedGeneration": observed,
                    "conditions": [{"type": "Ready", "status": "True"}]}}))
        };
        assert_eq!(
            check_progress(&check, &resource, &[node(1)], now),
            Progress::Waiting
        );
        assert_eq!(
            check_progress(&check, &resource, &[node(2)], now),
            Progress::Ready
        );
    }

    #[test]
    fn a_check_timeout_reads_seconds_and_minutes() {
        assert_eq!(timeout_seconds("60s"), 60);
        assert_eq!(timeout_seconds("5m"), 300);
        assert_eq!(timeout_seconds("soon"), 300);
    }

    #[test]
    fn a_selector_joins_its_labels() {
        let labels = BTreeMap::from([
            ("app".to_owned(), "traefik".to_owned()),
            ("tier".to_owned(), "edge".to_owned()),
        ]);
        assert_eq!(selector(&labels), "app=traefik,tier=edge");
    }
}
