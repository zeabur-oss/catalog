package catalog

import (
	"maps"
	"slices"
	"strings"
	"time"

	gen "github.com/invopop/jsonschema"
)

// Wait is the objects that a check waits for, and what they reach. Health checks, the smoke checks
// of components and the smoke checks of templates all run as a Wait.
type Wait struct {
	// One object as kind/name, such as certificate/smoke. A kind alone takes a selector or all.
	Resource string `yaml:"resource" json:"resource" jsonschema:"required,pattern=^[A-Za-z][A-Za-z0-9.]*(/[a-z0-9][a-z0-9.-]*)?$"`
	// The namespace of the objects. A kind without namespaces has none.
	Namespace string `yaml:"namespace" json:"namespace,omitempty" jsonschema:"minLength=1"`
	// The labels of the objects, each a key with its value.
	Selector map[string]string `yaml:"selector" json:"selector,omitempty"`
	// Every object of the kind.
	All bool `yaml:"all" json:"all,omitempty"`
	// What the objects reach, as a kubectl wait --for value. A deployment, statefulset or daemonset
	// has none: it is ready when its rollout is complete.
	Condition string `yaml:"condition" json:"condition,omitempty"`
}

func (Wait) JSONSchemaExtend(s *gen.Schema) {
	SetCondition(s)
	selector, _ := s.Properties.Get("selector")
	selector.MinProperties = new(uint64(1))
	all, _ := s.Properties.Get("all")
	all.Const = true
	// A kind alone needs a selector or all, and one object takes neither.
	named := gen.NewProperties()
	named.Set("resource", &gen.Schema{Pattern: "/"})
	s.If = &gen.Schema{Properties: named}
	s.Then = &gen.Schema{Not: &gen.Schema{AnyOf: []*gen.Schema{{Required: []string{"selector"}}, {Required: []string{"all"}}}}}
	s.Else = &gen.Schema{OneOf: []*gen.Schema{{Required: []string{"selector"}}, {Required: []string{"all"}}}}
	// A workload is ready when its rollout is complete, so it needs no condition, and a condition
	// of a workload is a jsonpath on its fields. Any other object needs one, because nothing else
	// says when it is ready.
	workload := gen.NewProperties()
	workload.Set("resource", &gen.Schema{Pattern: WorkloadResource()})
	jsonpathOnly := gen.NewProperties()
	jsonpathOnly.Set("condition", &gen.Schema{Pattern: "^jsonpath="})
	s.AllOf = []*gen.Schema{{
		If:   &gen.Schema{Properties: workload},
		Then: &gen.Schema{Properties: jsonpathOnly},
		Else: &gen.Schema{Required: []string{"condition"}},
	}}
}

// Target returns the objects of w. A Wait that has no namespace uses defaultNS.
func (w Wait) Target(defaultNS string) Target {
	kind, name, _ := strings.Cut(w.Resource, "/")
	t := Target{Resource: kind, Namespace: w.Namespace, Name: name}
	if t.Namespace == "" {
		t.Namespace = defaultNS
	}
	for _, k := range slices.Sorted(maps.Keys(w.Selector)) {
		t.Selector += "," + k + "=" + w.Selector[k]
	}
	t.Selector = strings.TrimPrefix(t.Selector, ",")
	return t
}

// Args returns the arguments of the kubectl wait for the condition of w.
func (w Wait) Args(defaultNS, timeout string) []string {
	t, args := w.Target(defaultNS), []string{"wait", "--for=" + w.Condition}
	switch {
	case t.Name != "":
		args = append(args, t.Resource+"/"+t.Name)
	case t.Selector != "":
		args = append(args, t.Resource, "-l", t.Selector)
	default:
		args = append(args, t.Resource, "--all")
	}
	if t.Namespace != "" {
		args = append(args, "-n", t.Namespace)
	}
	return append(args, "--timeout="+timeout)
}

// Describe returns the arguments that select the objects of w for kubectl describe, and their
// namespace.
func (w Wait) Describe(defaultNS string) (object []string, ns string) {
	t := w.Target(defaultNS)
	switch {
	case t.Name != "":
		object = []string{t.Resource + "/" + t.Name}
	case t.Selector != "":
		object = []string{t.Resource, "-l", t.Selector}
	default:
		object = []string{t.Resource}
	}
	if t.Namespace != "" {
		object = append([]string{"-n", t.Namespace}, object...)
	}
	return object, t.Namespace
}

// Run waits until the objects of w are ready: the rollout of a workload, the condition of any other
// object. The objects may not exist yet.
func (w Wait) Run(kubectl func(args ...string) (string, error), defaultNS, timeout string) error {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return err
	}
	t := w.Target(defaultNS)
	if w.Condition == "" {
		return WaitReady(kubectl, d, t)
	}
	if err := WaitObjects(kubectl, d, t); err != nil {
		return err
	}
	_, err = kubectl(w.Args(defaultNS, timeout)...)
	return err
}
