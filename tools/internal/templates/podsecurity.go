package templates

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

var podSecurity = func() policy.Evaluator {
	e, err := policy.NewEvaluator(policy.DefaultChecks(), nil)
	if err != nil {
		panic(err)
	}
	return e
}()

// checkPodSecurity evaluates a pod with the upstream Pod Security Admission
// checks at level (baseline or restricted), latest version.
func checkPodSecurity(meta, spec map[string]any, level, w string, fail func(string, Rule, ...string)) {
	var pm metav1.ObjectMeta
	var ps corev1.PodSpec
	if err := convert(meta, &pm); err != nil {
		fail(w, ErrPodSecurity, err.Error())
		return
	}
	if err := convert(spec, &ps); err != nil {
		fail(w, ErrPodSecurity, err.Error())
		return
	}
	lv := api.LevelVersion{Level: api.Level(level), Version: api.LatestVersion()}
	res := policy.AggregateCheckResults(podSecurity.EvaluatePod(lv, &pm, &ps))
	if !res.Allowed {
		fail(w, ErrPodSecurity, level+": "+res.ForbiddenDetail())
	}
}

func convert(in map[string]any, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
