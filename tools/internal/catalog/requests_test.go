package catalog

import "testing"

func TestMemoryRequests(t *testing.T) {
	const Mi = 1 << 20
	cases := []struct {
		name     string
		manifest string
		want     int64
	}{
		{"containers add up", `kind: Deployment
metadata: {name: a}
spec:
  template:
    spec:
      containers:
        - {name: a, resources: {requests: {memory: 100Mi}}}
        - {name: b, resources: {requests: {memory: 0.5Gi}}}
`, 612 * Mi},
		{"init larger than containers", `kind: Deployment
metadata: {name: a}
spec:
  template:
    spec:
      initContainers:
        - {name: i, resources: {requests: {memory: 300Mi}}}
      containers:
        - {name: a, resources: {requests: {memory: 100Mi}}}
`, 300 * Mi},
		{"init smaller than containers", `kind: Deployment
metadata: {name: a}
spec:
  template:
    spec:
      initContainers:
        - {name: i, resources: {requests: {memory: 10Mi}}}
      containers:
        - {name: a, resources: {requests: {memory: 100Mi}}}
`, 100 * Mi},
		{"sidecar adds to containers and to later init containers", `kind: StatefulSet
metadata: {name: a}
spec:
  replicas: 1
  template:
    spec:
      initContainers:
        - {name: s, restartPolicy: Always, resources: {requests: {memory: 50Mi}}}
        - {name: i, resources: {requests: {memory: 200Mi}}}
      containers:
        - {name: a, resources: {requests: {memory: 100Mi}}}
`, 250 * Mi},
		{"replicas multiply", `kind: StatefulSet
metadata: {name: a}
spec:
  replicas: 3
  template:
    spec:
      containers:
        - {name: a, resources: {requests: {memory: 64Mi}}}
`, 192 * Mi},
		{"zero replicas", `kind: Deployment
metadata: {name: a}
spec:
  replicas: 0
  template:
    spec:
      containers:
        - {name: a, resources: {requests: {memory: 64Mi}}}
`, 0},
		{"daemonset counts once", `kind: DaemonSet
metadata: {name: a}
spec:
  template:
    spec:
      containers:
        - {name: a, resources: {requests: {memory: 32Mi}}}
`, 32 * Mi},
		{"missing requests count as zero", `kind: Deployment
metadata: {name: a}
spec:
  template:
    spec:
      containers:
        - {name: a, resources: {requests: {cpu: 10m}}}
        - {name: b}
`, 0},
		{"limit stands in for a missing request", `kind: Deployment
metadata: {name: a}
spec:
  template:
    spec:
      containers:
        - {name: a, resources: {limits: {memory: 128Mi}}}
        - {name: b, resources: {requests: {memory: 16Mi}, limits: {memory: 128Mi}}}
`, 144 * Mi},
		{"jobs, hooks and other kinds are skipped", `kind: Job
metadata: {name: a}
spec:
  template:
    spec:
      containers:
        - {name: a, resources: {requests: {memory: 64Mi}}}
---
kind: CronJob
metadata: {name: b}
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - {name: a, resources: {requests: {memory: 64Mi}}}
---
kind: Pod
metadata: {name: c, annotations: {helm.sh/hook: test}}
spec:
  containers:
    - {name: a, resources: {requests: {memory: 64Mi}}}
---
kind: ConfigMap
metadata: {name: d}
`, 0},
		{"quantity units", `kind: Pod
metadata: {name: a}
spec:
  containers:
    - {name: a, resources: {requests: {memory: 1G}}}
    - {name: b, resources: {requests: {memory: 128M}}}
    - {name: c, resources: {requests: {memory: 1048576}}}
    - {name: d, resources: {requests: {memory: 1Gi}}}
`, 1_000_000_000 + 128_000_000 + 1<<20 + 1<<30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := MemoryRequests([]byte(c.manifest))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %d bytes, want %d", got, c.want)
			}
		})
	}
}

func TestMemoryRequestsRejectsBadQuantity(t *testing.T) {
	_, err := MemoryRequests([]byte(`kind: Pod
metadata: {name: a}
spec:
  containers:
    - {name: a, resources: {requests: {memory: lots}}}
`))
	if err == nil {
		t.Error("want an error for an unparsable quantity")
	}
}

func TestMiBRoundsUp(t *testing.T) {
	for in, want := range map[int64]int64{0: 0, 1: 1, 1 << 20: 1, 1<<20 + 1: 2} {
		if got := MiB(in); got != want {
			t.Errorf("MiB(%d) = %d, want %d", in, got, want)
		}
	}
}
