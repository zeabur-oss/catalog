package catalog

import (
	"errors"
	"strings"
	"testing"
)

func TestDiagnoseIncludesPodLogs(t *testing.T) {
	run := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "get pods -o name"):
			return "pod/db-0\n", nil
		case strings.Contains(joined, "--previous"):
			return "crashed last time", nil
		case strings.Contains(joined, "logs pod/db-0"):
			return "starting", nil
		case strings.Contains(joined, "describe"):
			return "", errors.New("gone")
		}
		return "ok", nil
	}
	got := Diagnose(run, "ns", "-n", "ns", "db/smoke")
	for _, want := range []string{"logs of pod/db-0\nstarting", "previous logs of pod/db-0\ncrashed last time", "gone"} {
		if !strings.Contains(got, want) {
			t.Errorf("diagnosis lacks %q:\n%s", want, got)
		}
	}
}
