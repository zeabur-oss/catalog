## Introduction

Trivy Operator scans the images of running workloads for known vulnerabilities. It also audits workload settings and looks for exposed secrets. It writes the results as reports in the workload's own namespace: `VulnerabilityReport`, `ConfigAuditReport` and `ExposedSecretReport`.

Install it when you want a list of vulnerable images in the cluster. The scan Jobs use noticeable CPU and memory, so the component is optional. It lists only vulnerabilities that have a fix. On a single node it scans one image at a time, reports MEDIUM, HIGH and CRITICAL findings, and skips the RBAC review.

It reads workloads, Pods, Services, Ingresses and RBAC objects in every namespace, runs scan Jobs and writes the reports. It does not read every Secret. It leaves infrastructure and compliance scans and SBOM reports off.

## Use it

Trivy Operator has no Zeabur screen. Read the reports with kubectl once the first scans finish.

```sh
kubectl get vulnerabilityreports -A -o wide
kubectl get configauditreports -A -o wide
```

See the [Trivy Operator documentation](https://aquasecurity.github.io/trivy-operator/) for the report fields and settings.

## Remove it

Removal deletes the operator. The CRDs stay, and so do the reports already written. Delete them yourself if you no longer want them. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### No reports appear

Read the operator log and look for scan Jobs that fail:

```sh
kubectl -n trivy-system logs deploy/trivy-operator
kubectl -n trivy-system get jobs,pods
```
