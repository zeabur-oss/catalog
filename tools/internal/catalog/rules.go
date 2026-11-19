package catalog

import (
	"strings"

	gen "github.com/invopop/jsonschema"
)

// WorkloadKinds roll out their pods. Their readiness is the rollout, so a check of one names no
// condition.
var WorkloadKinds = []string{"DaemonSet", "Deployment", "StatefulSet"}

// TimeoutPattern is a timeout in whole seconds or minutes, above zero.
const TimeoutPattern = `^[1-9][0-9]*(s|m)$`

// ConditionPattern is a kubectl wait --for value: a condition, or a jsonpath.
const ConditionPattern = `^(condition=[A-Za-z][A-Za-z0-9]*(=(True|False|Unknown))?|jsonpath=.+)$`

// WorkloadResource is the pattern of a kubectl resource that names a workload, as a kind alone or
// as kind/name. It takes the kind with any letter in either case, such as deployment or StatefulSet.
func WorkloadResource() string {
	kinds := make([]string, len(WorkloadKinds))
	for i, k := range WorkloadKinds {
		var b strings.Builder
		for _, r := range k {
			if lower := strings.ToLower(string(r)); lower != string(r) {
				b.WriteString("[" + string(r) + lower + "]")
			} else {
				b.WriteRune(r)
			}
		}
		kinds[i] = b.String()
	}
	return "^(" + strings.Join(kinds, "|") + ")(/|$)"
}

// setPattern sets the pattern of a property that the struct tag leaves open.
func setPattern(s *gen.Schema, property, pattern string) {
	p, _ := s.Properties.Get(property)
	p.Pattern = pattern
}

// SetTimeout gives the timeout property of s the TimeoutPattern.
func SetTimeout(s *gen.Schema) { setPattern(s, "timeout", TimeoutPattern) }

// SetCondition gives the condition property of s the ConditionPattern.
func SetCondition(s *gen.Schema) { setPattern(s, "condition", ConditionPattern) }
