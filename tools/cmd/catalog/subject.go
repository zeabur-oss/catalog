package main

import (
	"errors"
	"log/slog"
	"runtime"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// The measurement directory of a component and of a template.
const (
	componentsDir = "components"
	templatesDir  = "templates"
)

// subject is what a test runs on: a component or a template.
type subject interface {
	ID() string
	// Source is the directory whose digest is the subject's input.
	Source() string
	// Order is what testing the subject installs on profile, in order.
	Order(profile string) ([]*catalog.Component, error)
	// Measurement is the directory its measurements are kept in, and the file
	// that records a run on arch, empty when none is.
	Measurement(arch string) (dir, path string)
	// Run tests the subject on the current cluster.
	Run(t *catalog.Tester, o runSpec) (status string, err error)
}

// runSpec is how a subject runs.
type runSpec struct {
	cfg     config
	profile string
	in      Environment
	write   bool // record the result
	// smoke runs the quick test of the smoke tier, which records nothing;
	// keep leaves it installed for the subjects after it.
	smoke bool
	keep  bool
	// setupErr is why the cluster for the run could not be made.
	setupErr error
}

// subjectOf returns the component or template with id.
func (r *repo) subjectOf(id string) (subject, bool) {
	if c, ok := r.cat.Get(id); ok {
		return componentSubject{r, c}, true
	}
	if t, err := templates.Get(r.templates, id); err == nil {
		return templateSubject{r, t}, true
	}
	return nil, false
}

type componentSubject struct {
	r *repo
	c *catalog.Component
}

func (s componentSubject) ID() string     { return s.c.ID }
func (s componentSubject) Source() string { return s.c.Dir() }

func (s componentSubject) Order(profile string) ([]*catalog.Component, error) {
	return s.r.cat.InstallOrder(catalog.ByID(s.c.ID), profile)
}

func (s componentSubject) Measurement(string) (string, string) {
	return componentsDir, measurementPath(s.c.ID)
}

// Run tests the component. If write is set, it writes the measurement under
// the profile with the source digests of in, or marks the environment failed.
func (s componentSubject) Run(t *catalog.Tester, o runSpec) (string, error) {
	if o.smoke {
		slog.Info("testing", "component", s.c.ID)
		return "", t.Quick(s.c.ID, o.keep)
	}
	fail := func(err error, env *Environment) error {
		if !o.write {
			return err
		}
		return errors.Join(err, markFailed(o.cfg, o.profile, runtime.GOARCH, s.c, env, o.in))
	}
	if o.setupErr != nil {
		return "", fail(o.setupErr, nil)
	}
	env, err := currentEnvironment(o.cfg, o.profile)
	if err != nil {
		return "", fail(err, nil)
	}
	slog.Info("testing", "component", s.c.ID, "distribution", env.Distribution, "platform", env.Platform.String(), "kubernetes", env.KubernetesVersion)
	res := t.Test(s.c.ID)
	if res.Err != nil {
		return "", fail(res.Err, &env)
	}
	if res.Unmeasured {
		return "PASS (installed without credentials; not measured)", nil
	}
	if o.write {
		if err := writeMeasurement(s.c, env, o.in, res); err != nil {
			return "", err
		}
	}
	return podSummary(res.Pods), nil
}

type templateSubject struct {
	r *repo
	t *templates.Template
}

func (s templateSubject) ID() string     { return s.t.ID }
func (s templateSubject) Source() string { return s.t.Dir }

func (s templateSubject) Order(profile string) ([]*catalog.Component, error) {
	refs, err := templateRefs(s.r, s.t)
	if err != nil {
		return nil, err
	}
	return s.r.cat.InstallOrder(refs, profile)
}

// Measurement is empty off templateArch, where templates are not run.
func (s templateSubject) Measurement(arch string) (string, string) {
	if arch != templateArch {
		return templatesDir, ""
	}
	return templatesDir, templateMeasurementPath(s.t.ID)
}

// Run tests the template. If write is set, it writes the measurement with the
// source digests of in.
func (s templateSubject) Run(t *catalog.Tester, o runSpec) (string, error) {
	if o.setupErr != nil {
		return "", o.setupErr
	}
	if o.smoke {
		_, err := runTemplate(s.r, t, s.t.ID, false)
		return "", err
	}
	env, err := currentEnvironment(o.cfg, t.Profile)
	if err != nil {
		return "", err
	}
	usage, err := runTemplate(s.r, t, s.t.ID, true)
	if err != nil {
		return "", err
	}
	if o.write {
		values, err := e2eValues(s.r, s.t)
		if err != nil {
			return "", err
		}
		manifest, err := templates.E2EManifest(s.t, values)
		if err != nil {
			return "", err
		}
		images, err := imagesOf(manifest)
		if err != nil {
			return "", err
		}
		if err := writeTemplateMeasurement(s.t, env, o.in, usage, images); err != nil {
			return "", err
		}
	}
	return podSummary(usage), nil
}
