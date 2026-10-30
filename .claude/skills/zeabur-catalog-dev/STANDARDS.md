# Engineering standards: Zeabur Catalog's part

Apply `zeabur-oss-standards`' STANDARDS.md first. Zeabur Catalog adds these items to its
sections.

## 1. Comments, doc comments and commit messages

```go
// Before: If lease expired, take it and start reconcile loop.
// After:  If the Lease is expired, take the Lease. Then start the reconcile loop.

// Before: Render is used for performing the rendering of the chart, which must have values.
// After:  Render renders the chart with the given values.
```

```yaml
# Before: bumping this since it was OOMing under load
# After:  The controller used 400 MiB with 2,000 Certificates. This limit keeps a margin of 50%.
```

```text
Before: Updated the cronjob stuff so old jobs dont pile up
After:  fix(databases): keep three finished backup Jobs

        The CronJob kept every finished Job. Each Job and its Pod stayed in etcd.
```

## 2. Tests

```go
// Before: three copies of one test with different input
func TestRejectEmpty(t *testing.T) { … }
func TestRejectSpaces(t *testing.T) { … }
// After
for _, c := range []struct{ name, in string }{{"empty", ""}, {"spaces", " "}} {
    t.Run(c.name, func(t *testing.T) { if valid(c.in) { t.Fatal("accepted") } })
}
```

## 3. Design

Leave alone also: consumer-side Go interfaces, error types.

```go
// Before: an interface and a registry for one renderer
type Renderer interface{ Render(Chart, Values) ([]byte, error) }
var renderers = map[string]Renderer{"helm": helmRenderer{}}
// After
out, err := renderHelm(chart, values)
```

## 4. Scale

The catalogue ships manifests and values, not a controller of its own: of the shared scale
rules, "Objects and size", "Observability" and the fan-out bound apply, plus these.

**Manifests and values**
- One operator per cluster reconciles every instance of its kind. Do not ship a controller,
  Deployment or CronJob per tenant item. A CronJob can run zero or two times, so its Job is
  idempotent.
- A workload that keeps required state in memory or on its own volume runs one replica, or the
  record shows the shared store that lets it run more.
- A value that ends up in one object stays under the object limits.

```yaml
# Before: a CronJob per database, every finished Job kept
# After:  one CronJob per cluster that backs up every database it selects
successfulJobsHistoryLimit: 3
failedJobsHistoryLimit: 1
jobTemplate:
  spec:
    ttlSecondsAfterFinished: 86400
```

Leave alone also: one Deployment per real application; one Job per real backup run; a value far
below the thresholds.
