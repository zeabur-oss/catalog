package catalog

import (
	"fmt"
	"strings"
)

// Diagnose reports what a wait gave up on. describe is the arguments that
// select the object, such as "-n ns mariadb/smoke", and ns is its namespace.
// It adds the last log lines of every pod, and of its previous containers.
// Each part that kubectl cannot fetch is left out.
func Diagnose(run func(args ...string) (string, error), ns string, describe ...string) string {
	var b strings.Builder
	section := func(title string, args ...string) {
		out, err := run(args...)
		if err != nil {
			out = err.Error()
		}
		fmt.Fprintf(&b, "\n--- %s\n%s\n", title, strings.TrimSpace(out))
	}
	if len(describe) > 0 {
		section("kubectl describe "+strings.Join(describe, " "), append([]string{"describe"}, describe...)...)
	}
	if ns != "" {
		section("pods in "+ns, "-n", ns, "get", "pods", "-o", "wide")
		section("events in "+ns, "-n", ns, "get", "events", "--sort-by=.lastTimestamp")
		pods, _ := run("-n", ns, "get", "pods", "-o", "name")
		for _, pod := range strings.Fields(pods) {
			section("logs of "+pod, "-n", ns, "logs", pod, "--all-containers", "--prefix", "--ignore-errors", "--tail=40")
			if out, err := run("-n", ns, "logs", pod, "--all-containers", "--prefix", "--ignore-errors", "--tail=40", "--previous"); err == nil && strings.TrimSpace(out) != "" {
				fmt.Fprintf(&b, "\n--- previous logs of %s\n%s\n", pod, strings.TrimSpace(out))
			}
		}
	}
	return b.String()
}

// waitTarget splits the arguments of a kubectl wait into the object that
// describe takes and its namespace, without the --for and --timeout flags.
func waitTarget(fields []string) (object []string, ns string) {
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "-n" && i+1 < len(fields):
			ns = fields[i+1]
			object = append(object, f, ns)
			i++
		case strings.HasPrefix(f, "--for=") || strings.HasPrefix(f, "--timeout="):
		default:
			object = append(object, f)
		}
	}
	return
}
