# Engineering standards

Four checks for every change. Each item is a fail condition a reviewer can point at. The
"Leave alone" lists stop over-correction: a finding must not land on them.

## 1. Comments, doc comments and commit messages

Prose follows ASD-STE100 Issue 9 (Simplified Technical English), adapted to code. Code spans,
identifiers, Kubernetes kinds and fields, numbers with units and RFC designators are technical
nouns: each counts as one word and is never reworded.

- Classify each sentence. A step the reader does is **procedural**: imperative, at most 20
  words. Everything else is **descriptive**: at most 25 words; a paragraph has at most six
  sentences and one topic.
- One instruction per sentence. Put sequential steps in a numbered list.
- Condition first: "If the Lease is expired, take it."
- Active voice with a named actor: "The API server rejects the write." Use the passive only when
  the actor is unknown.
- Simple tenses: present, past, future, imperative. Write "watches", not "is watching" or "has
  watched".
- A verb for an action: "Delete the Pod", not "perform the deletion of the Pod".
- An `-ing` word only as a technical noun (`Pending`, "rolling update") or in "during",
  "missing", "remaining".
- Full sentences: keep articles and verbs, write "does not" for "doesn't", and use periods, not
  semicolons.
- Noun clusters of at most three words: "the cache of role bindings for workspace members".
- One term per concept and one meaning per term: the term the code and docs already use.
- A single verb when one exists: "configure", not "set up".
- `NOTE:` states a fact. Write an instruction as its own imperative sentence.
- `WARNING:` only on a destructive operation (data loss, a security boundary, an outage): the
  condition or command first, then the concrete result.
- Keep the spelling the repository already uses; do not mix British and American forms.
- Commit subject: the Conventional Commits prefix, then one imperative instruction of at most 20
  words. Body: descriptive sentences of at most 25 words that say why.

Leave alone: code, literals, log keys and metric names; quoted upstream text, licence headers and
generated files; tags and labels (`# Safety`, `SAFETY:`, `TODO(name):`, `@param`); RFC 2119
MUST/SHOULD in a contract; comments the change does not touch. These rules govern how a comment
reads, not whether to write one.

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

A test earns its place when it fails because a promise broke, and only then. Ask four questions
(Khorikov): does it catch a real regression, survive a refactor that keeps the behaviour, run
fast, and read clearly?

Delete or rewrite a test that:
- restates the implementation: an expected value copied from the code, a mocked sequence of
  internal calls, a table, `match` or `switch` copied into the test (a change-detector);
- asserts internal interactions (which helper, a harmless call order) where output or state is
  observable. Keep a call count or order that is itself the requirement;
- reaches private code through a test-only export when the public interface shows the result;
- tests the language, a framework, a library or generated code, with no behaviour of ours;
- tests trivial code: getters, constants, `String()` methods, plain struct tags;
- pins exact log text, a full error sentence or map order that no caller reads;
- is a large snapshot or golden file that people regenerate without reading;
- passes when the result is wrong (no assertion, or only "did not fail").

Merge tests that share setup and action and differ only in input into one table of named cases (`t.Run` over a slice of cases).
Split a test that checks unrelated behaviours; several assertions about one outcome are one
behaviour.

Move a rule tested at several levels to the lowest level that can see it. Keep a higher test only
for what that level cannot see: wiring, a real API server, a real browser. A test that uses the
network, disk, a sleep or a cluster belongs in the suite for its size; do not delete it for being
slow.

Before you remove a test, name the promise it protects and the test that still protects it. A
mutation report that shows no unique kill is a reason to look, not proof.

Leave alone: security tests repeated at levels that each see a different failure; published
contracts (API fields, CLI output, CRD schema, rendered manifests); mocks at real out-of-process
boundaries (SMTP, an OIDC provider, a Git host); regression tests of fixed bugs; small reviewed
golden files; property and fuzz tests; coverage or test count on its own.

```go
// Before: three copies of one test with different input
func TestRejectEmpty(t *testing.T) { … }
func TestRejectSpaces(t *testing.T) { … }
// After
for _, c := range []struct{ name, in string }{{"empty", ""}, {"spaces", " "}} {
    t.Run(c.name, func(t *testing.T) { if valid(c.in) { t.Fatal("accepted") } })
}
```

## 3. Design: build for a caller that exists

Flag a structure when no present caller can be named:
- a trait or interface with one production implementation and no real boundary behind it;
- a generic parameter or dynamic dispatch that serves one concrete type;
- a parameter, option, flag, config key or hook that no production code sets;
- a wrapper or layer that forwards the same arguments and result and adds no invariant
  (a pass-through);
- a configuration knob for a value the code can compute;
- a registry, plugin system, DSL or rules engine with one use;
- shared code that grows a mode flag per caller: inline it, delete each caller's unused branches,
  then extract what is really shared (Metz, "The Wrong Abstraction");
- an abstraction drawn from one or two call sites that do not match yet: wait for the third. A
  duplicated security check is the exception: give it one implementation now;
- a second copy of what Kubernetes provides: a permission model, an object store, a scheduler
  or retry queue, a query language;
- an internal API with versions or adapters and one consumer that ships with it;
- a large design with no working simple path first (Gall).

A finding names the cost, the missing caller, and the simpler design that keeps the behaviour.

Leave alone: a seam at a real out-of-process boundary; layers that enforce security or
correctness (plan, review, apply; authorization checks); published interfaces (API, CRDs,
documented Helm values and flags: removing one is a deprecation, not a cleanup); consumer-side Go interfaces, error types; the second copy of a block; a deep module that hides much
behind a small interface; essential Kubernetes semantics (conflicts, finalizers, owner
references).

```go
// Before: an interface and a registry for one renderer
type Renderer interface{ Render(Chart, Values) ([]byte, error) }
var renderers = map[string]Renderer{"helm": helmRenderer{}}
// After
out, err := renderHelm(chart, values)
```

## 4. Scale: no unbounded cost on day one

Fail a design when its cost per request, item, tenant, tab or replica grows with no bound, or
when a second replica breaks it. A finding names the growth variable, the resource and the
missing bound.

**Manifests and values**
- One operator per cluster reconciles every instance of its kind. Do not ship a controller,
  Deployment or CronJob per tenant item. A CronJob can run zero or two times, so its Job is
  idempotent.
- A workload that keeps required state in memory or on its own volume runs one replica, or the
  record shows the shared store that lets it run more.
- A value that ends up in one object stays under the object limits below.

**Objects and size**
- Hard limits: an etcd request 1.5 MiB (default), a Secret or ConfigMap 1 MiB, all annotations of
  an object 256 KiB. Logs, plans and large blobs go elsewhere.
- Soft thresholds (sig-scalability): 150,000 objects of one type; per namespace 3,000 Pods,
  2,000 Deployments, 5,000 Services; 10,000 namespaces; Pods per node min(110, 10 × cores).
  A design far below them passes; growth with no bound fails.
- Everything stored has a retention bound: Job `ttlSecondsAfterFinished`, CronJob
  `successfulJobsHistoryLimit` and `failedJobsHistoryLimit`, a cap on history objects.
- A Kubernetes label identifies a set that something selects. A Prometheus label takes a bounded
  set of values.

**Tooling**
- Bound fan-out: `errgroup.SetLimit(n)` before `g.Go`, or a buffered channel as a semaphore.

**Observability**
- A metric label takes a bounded set of values (aim under 10, look again over 100): no person,
  project or object IDs.
- No log line per object per resync at info level, and no full object dumps.

```yaml
# Before: a CronJob per database, every finished Job kept
# After:  one CronJob per cluster that backs up every database it selects
successfulJobsHistoryLimit: 3
failedJobsHistoryLimit: 1
jobTemplate:
  spec:
    ttlSecondsAfterFinished: 86400
```

Leave alone: one Deployment per real application; one Job per real backup run; a namespace per
project; a value far below the thresholds; polling an external system that has no watch API,
with backoff; the cost of fail-closed admission and RBAC.
