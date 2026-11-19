package catalog

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func deployment(generation, observed, updated, ready int) string {
	return fmt.Sprintf(`{
 "apiVersion": "apps/v1", "kind": "Deployment",
 "metadata": {"name": "web", "namespace": "ns", "generation": %d},
 "spec": {"replicas": 2},
 "status": {"observedGeneration": %d, "replicas": 2, "updatedReplicas": %d, "readyReplicas": %d, "availableReplicas": %d,
  "conditions": [{"type": "Available", "status": "True"}, {"type": "Progressing", "status": "True", "reason": "NewReplicaSetAvailable"}]}
}`, generation, observed, updated, ready, ready)
}

// replies gives the answers of kubectl get in order and then repeats the last.
func replies(answers ...string) func(...string) (string, error) {
	i := 0
	return func(...string) (string, error) {
		a := answers[min(i, len(answers)-1)]
		i++
		return a, nil
	}
}

func TestWaitReadyWaitsForTheLatestRollout(t *testing.T) {
	pollEvery = time.Millisecond
	target := Target{Resource: "deployments.apps", Namespace: "ns", Name: "web"}
	// The old ReplicaSet is Available while the new generation has not started.
	get := replies("", deployment(2, 1, 0, 2), deployment(2, 2, 1, 1), deployment(2, 2, 2, 2))
	if err := WaitReady(get, time.Minute, target); err != nil {
		t.Fatal(err)
	}
}

func TestWaitReadyReportsWhatKeepsAnObjectFromReady(t *testing.T) {
	pollEvery = time.Millisecond
	target := Target{Resource: "deployments.apps", Namespace: "ns", Name: "web"}
	err := WaitReady(replies(deployment(2, 1, 0, 2)), 20*time.Millisecond, target)
	if err == nil || !strings.Contains(err.Error(), "Deployment/web") || !strings.Contains(err.Error(), "not ready after") {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitReadyFailsAtOnceOnAFailedObject(t *testing.T) {
	pollEvery = time.Hour
	job := `{"apiVersion": "batch/v1", "kind": "Job", "metadata": {"name": "j", "namespace": "ns"},
 "status": {"conditions": [{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded", "message": "Job has reached the specified backoff limit"}]}}`
	err := WaitReady(replies(job), time.Hour, Target{Resource: "jobs.batch", Name: "j"})
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitReadyKeepsWaitingWhenKubectlErrors(t *testing.T) {
	pollEvery = time.Millisecond
	calls := 0
	get := func(...string) (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("the server doesn't have a resource type")
		}
		return deployment(1, 1, 2, 2), nil
	}
	if err := WaitReady(get, time.Minute, Target{Resource: "deployments.apps", Name: "web"}); err != nil {
		t.Fatal(err)
	}
}

func TestWaitObjects(t *testing.T) {
	pollEvery = time.Millisecond
	get := replies("", "", "deployment.apps/web\n")
	if err := WaitObjects(get, time.Minute, Target{Resource: "deployments.apps", Name: "web"}); err != nil {
		t.Fatal(err)
	}
	if err := WaitObjects(replies(""), 5*time.Millisecond, Target{Resource: "pods"}); err == nil {
		t.Error("want an error when no object appears")
	}
}
