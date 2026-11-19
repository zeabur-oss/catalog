package catalog

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"sigs.k8s.io/cli-utils/pkg/kstatus/status"

	"github.com/zeabur-oss/catalog/tools/internal/kubeobj"
)

// Target names the objects of one kubectl resource that a wait is about. Without a Name and a
// Selector it is every object of the resource.
type Target struct {
	Resource  string // as kubectl names it, such as deployments.apps
	Namespace string
	Name      string
	Selector  string // a label selector, such as app=web,tier=api
}

func (t Target) String() string {
	s := t.Resource
	if t.Name != "" {
		s += "/" + t.Name
	}
	if t.Selector != "" {
		s += " -l " + t.Selector
	}
	if t.Namespace != "" {
		s += " -n " + t.Namespace
	}
	return s
}

// getArgs returns the kubectl arguments that print the objects of t as JSON. kubectl prints
// nothing for an object that does not exist.
func (t Target) getArgs(output string) []string {
	args := []string{"get", t.Resource}
	if t.Name != "" {
		args = append(args, t.Name)
	}
	if t.Selector != "" {
		args = append(args, "-l", t.Selector)
	}
	if t.Namespace != "" {
		args = append(args, "-n", t.Namespace)
	}
	return append(args, "--ignore-not-found", "-o", output)
}

// pollEvery is how often WaitReady asks the cluster again.
var pollEvery = 2 * time.Second

// WaitReady waits until every object of every target is Current by the kstatus rule: a workload
// has rolled out its latest generation, a CRD is established, a Job is complete, and any other
// object reports no Reconciling or Stalled condition and is Ready when it has that condition.
// An object that does not exist yet is waited for. It fails at once on a Failed object, and
// after timeout with the reason and message of each object that is not ready.
func WaitReady(get func(args ...string) (string, error), timeout time.Duration, targets ...Target) error {
	deadline := time.Now().Add(timeout)
	pending := targets
	var failed []string
	for {
		var next []Target
		failed = failed[:0]
		for _, t := range pending {
			problems, done, err := check(get, t)
			if err != nil {
				return err
			}
			if !done {
				next = append(next, t)
				failed = append(failed, problems...)
			}
		}
		pending = next
		if len(pending) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("not ready after %s: %s", timeout, strings.Join(failed, "; "))
		}
		time.Sleep(pollEvery)
	}
}

// check reads the objects of t. It returns what keeps them from being ready, or done when they
// are. A Failed object is an error.
func check(get func(args ...string) (string, error), t Target) (problems []string, done bool, err error) {
	out, err := get(t.getArgs("json")...)
	if err != nil {
		// The resource itself may not exist yet, such as the kind of a CRD that is being installed.
		return []string{t.String() + ": " + err.Error()}, false, nil
	}
	objs, err := kubeobj.Decode([]byte(out))
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", t, err)
	}
	if len(objs) == 0 {
		return []string{t.String() + ": not found"}, false, nil
	}
	for _, o := range objs {
		res, err := status.Compute(o)
		if err != nil {
			return nil, false, fmt.Errorf("%s/%s: %w", o.GetKind(), o.GetName(), err)
		}
		switch res.Status {
		case status.CurrentStatus:
		case status.FailedStatus:
			return nil, false, errors.New(o.GetKind() + "/" + o.GetName() + " failed: " + res.Message)
		default:
			problems = append(problems, fmt.Sprintf("%s/%s %s: %s", o.GetKind(), o.GetName(), strings.ToLower(string(res.Status)), res.Message))
		}
	}
	return problems, len(problems) == 0, nil
}

// WaitObjects waits until every target has at least one object.
func WaitObjects(get func(args ...string) (string, error), timeout time.Duration, targets ...Target) error {
	deadline := time.Now().Add(timeout)
	for _, t := range targets {
		for {
			out, err := get(t.getArgs("name")...)
			if err == nil && strings.TrimSpace(out) != "" {
				break
			}
			if time.Now().After(deadline) {
				if err != nil {
					return fmt.Errorf("%s: %w", t, err)
				}
				return fmt.Errorf("%s: no object after %s", t, timeout)
			}
			time.Sleep(pollEvery)
		}
	}
	return nil
}
