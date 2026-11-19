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
		// helm test hooks run only on helm test, never at install.
		if strings.Contains(text(object(object(d["metadata"])["annotations"])["helm.sh/hook"]), "test") {
			continue
		}
		t, ok, err := podOf(d)
		where := text(d["kind"]) + "/" + text(object(d["metadata"])["name"])
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid pod: %w", where, err))
			continue
		}
		if !ok {
			continue
		}
		checkPodSecurity(t.ObjectMeta, t.Spec, level, where, fail)
	}
	return errors.Join(errs...)
}
