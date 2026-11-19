package templates

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
func checkPodSecurity(pm metav1.ObjectMeta, ps corev1.PodSpec, level, w string, fail func(string, Rule, ...string)) {
	lv := api.LevelVersion{Level: api.Level(level), Version: api.LatestVersion()}
	res := policy.AggregateCheckResults(podSecurity.EvaluatePod(lv, &pm, &ps))
	if !res.Allowed {
		fail(w, ErrPodSecurity, level+": "+res.ForbiddenDetail())
	}
}

// convert copies a map of an object into the Kubernetes type out.
func convert(in map[string]any, out any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(in, out)
}
