package catalog

import ()

// WorkloadKinds roll out their pods. Their readiness is the rollout, so a check of one names no
// condition.
var WorkloadKinds = []string{"DaemonSet", "Deployment", "StatefulSet"}

// TimeoutPattern is a timeout in whole seconds or minutes, above zero.
const TimeoutPattern = `^[1-9][0-9]*(s|m)$`

// ConditionPattern is a kubectl wait --for value: a condition, or a jsonpath.
const ConditionPattern = `^(condition=[A-Za-z][A-Za-z0-9]*(=(True|False|Unknown))?|jsonpath=.+)$`
