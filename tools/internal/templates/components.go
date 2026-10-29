package templates

import (
	"errors"
	"fmt"
	"strings"
)

// CheckPodSecurity evaluates every pod spec in a rendered component against
// a Pod Security level (baseline or restricted); an empty level
// (privileged) checks nothing.
func CheckPodSecurity(manifest []byte, level string) error {
	if level == "" {
		return nil
	}
	docs, err := DecodeDocuments(manifest)
	if err != nil {
		return err
	}
	var errs []error
	fail := func(where string, rule Rule, detail ...string) {
		errs = append(errs, fmt.Errorf("%s: %w: %s", where, rule, strings.Join(detail, " ")))
	}
	for _, d := range docs {
		k := text(d["kind"])
		// helm test hooks run only on helm test, never at install.
		if strings.Contains(text(object(object(d["metadata"])["annotations"])["helm.sh/hook"]), "test") {
			continue
		}
		spec := object(d["spec"])
		var pm, ps map[string]any
		switch k {
		case "Pod":
			pm, ps = object(d["metadata"]), spec
		case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
			t := object(spec["template"])
			pm, ps = object(t["metadata"]), object(t["spec"])
		case "CronJob":
			t := object(object(object(spec["jobTemplate"])["spec"])["template"])
			pm, ps = object(t["metadata"]), object(t["spec"])
		default:
			continue
		}
		checkPodSecurity(pm, ps, level, k+"/"+text(object(d["metadata"])["name"]), fail)
	}
	return errors.Join(errs...)
}
