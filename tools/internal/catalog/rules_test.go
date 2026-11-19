package catalog

import (
	"regexp"
	"testing"
)

func TestWorkloadResourceMatchesWorkloadsOnly(t *testing.T) {
	re := regexp.MustCompile(WorkloadResource())
	for _, s := range []string{"deployment", "Deployment/web", "statefulset/db", "StatefulSet", "daemonset/agent", "DaemonSet/agent"} {
		if !re.MatchString(s) {
			t.Errorf("%q is not a workload", s)
		}
	}
	for _, s := range []string{"deployments/web", "job/run", "certificate/smoke", "pod"} {
		if re.MatchString(s) {
			t.Errorf("%q is a workload", s)
		}
	}
}

func TestTimeoutPatternNeedsAUnitAndAboveZero(t *testing.T) {
	re := regexp.MustCompile(TimeoutPattern)
	for s, want := range map[string]bool{"180s": true, "5m": true, "0s": false, "00m": false, "10": false, "1h": false} {
		if re.MatchString(s) != want {
			t.Errorf("%q: match = %v, want %v", s, !want, want)
		}
	}
}
