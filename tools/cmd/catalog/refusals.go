package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// baseRender is the render of the pull request's base, made in the background.
type baseRender struct {
	sha  string // empty when there is nothing to upgrade from
	dir  string
	done chan error
}

// startBaseRender renders the merge-base of HEAD and the base branch with its
// own tools. The base branch is $CATALOG_BASE, origin/main by default.
func startBaseRender() *baseRender {
	b := &baseRender{done: make(chan error, 1)}
	ref := cmp.Or(os.Getenv("CATALOG_BASE"), "origin/main")
	sha, err := catalog.Run(nil, "git", "merge-base", "HEAD", ref)
	if err != nil {
		slog.Warn("no base to upgrade from", "ref", ref, "error", err)
		b.done <- nil
		return b
	}
	head, err := catalog.Run(nil, "git", "rev-parse", "HEAD")
	b.sha = strings.TrimSpace(sha)
	if err != nil || b.sha == strings.TrimSpace(head) {
		b.sha = ""
		b.done <- err
		return b
	}
	go func() { b.done <- b.render() }()
	return b
}

func (b *baseRender) render() error {
	src, err := os.MkdirTemp("", "catalog-base-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(src)
	archive := exec.Command("git", "archive", b.sha)
	extract := exec.Command("tar", "-x", "-C", src)
	extract.Stdin, err = archive.StdoutPipe()
	if err != nil {
		return err
	}
	if err := extract.Start(); err != nil {
		return err
	}
	if err := errors.Join(archive.Run(), extract.Wait()); err != nil {
		return fmt.Errorf("git archive %s: %w", b.sha, err)
	}
	if b.dir, err = filepath.Abs(filepath.Join("out", "render-base")); err != nil {
		return err
	}
	slog.Info("rendering the base", "sha", b.sha)
	cmd := exec.Command("go", "run", "./tools/cmd/catalog", "render", "-out", b.dir)
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("render %s: %w: %s", b.sha, err, tail(string(out), 20))
	}
	return nil
}

// wait returns the base render's directory for the Kubernetes version with
// the same minor as kv, or "" when there is no base.
func (b *baseRender) wait(kv string) (string, error) {
	if err := <-b.done; err != nil || b.sha == "" {
		return "", err
	}
	m, _ := filepath.Glob(filepath.Join(b.dir, minorVersion(kv)+".*"))
	if len(m) != 1 {
		return "", fmt.Errorf("the base renders no Kubernetes %s", minorVersion(kv))
	}
	return m[0], nil
}

func minorVersion(v string) string { return strings.Join(strings.SplitN(v, ".", 3)[:2], ".") }

// addsMajorChangeFile reports whether HEAD adds a change file under
// .changeset/ since base whose frontmatter says `default: major`.
func addsMajorChangeFile(dir, base string) (bool, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=A", base, "HEAD", "--", ".changeset/")
	cmd.Dir = dir
	names, err := cmd.Output()
	if err != nil {
		return false, err
	}
	for _, name := range strings.Fields(string(names)) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return false, err
		}
		lines := strings.Split(string(data), "\n")
		if lines[0] != "---" {
			continue
		}
		for _, line := range lines[1:] {
			if line == "---" {
				break
			}
			if strings.TrimSpace(line) == "default: major" {
				return true, nil
			}
		}
	}
	return false, nil
}

// refusal is an object the API server refused to update, and in which
// upgrade or profile switch.
type refusal struct {
	component string
	when      string
	upgrade   bool // an upgrade from the base, not a profile switch
	catalog.Refusal
}

// checkRefusals applies one render to the API server for real, sends the
// other with --dry-run=server, and fails on what the API server refuses: the
// two profiles in both directions, and the base's render of each profile
// replaced by dir's.
func checkRefusals(r *repo, dir string, base *baseRender) error {
	baseDir, err := base.wait(filepath.Base(dir))
	if err != nil {
		return err
	}
	comps := r.cat.Components
	found := make([][]refusal, len(comps))
	errs := make([]error, len(comps))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, c := range comps {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			found[i], errs[i] = componentRefusals(c, dir, baseDir)
		})
	}
	wg.Wait()
	major := false
	if base.sha != "" {
		if major, err = addsMajorChangeFile(".", base.sha); err != nil {
			return err
		}
	}
	return errors.Join(append(errs, judgeRefusals(slices.Concat(found...), major)...)...)
}

// judgeRefusals returns an error for each refusal. An upgrade refusal passes
// when the pull request adds a major change file, which says how to migrate; a
// profile switch never does.
func judgeRefusals(found []refusal, major bool) []error {
	var errs []error
	for _, x := range found {
		if x.upgrade && major {
			slog.Info("intended break", "component", x.component, "object", x.Kind+" "+x.Namespace+"/"+x.Name, "when", x.when)
			continue
		}
		verb := "switch"
		if x.upgrade {
			verb = "upgrade"
		}
		msg := fmt.Sprintf("%s: the API server refuses to %s %s %s/%s in place (%s) for %s. ",
			x.component, verb, x.Kind, x.Namespace, x.Name, strings.Join(x.Fields, ", "), x.when)
		if x.upgrade {
			msg += "Change the component so the object updates in place, or add a `major` change file with the migration steps."
		} else {
			msg += "Give the field the same value in both profiles."
		}
		errs = append(errs, errors.New(msg))
	}
	return errs
}

func componentRefusals(c *catalog.Component, dir, baseDir string) ([]refusal, error) {
	var out []refusal
	var errs []error
	keep := func(upgrade bool, when string, found []catalog.Refusal, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", c.ID, when, err))
		}
		for _, x := range found {
			out = append(out, refusal{c.ID, when, upgrade, x})
		}
	}
	a, b := catalog.Profiles[0], catalog.Profiles[1]
	if c.Defaults[a] != "unsupported" && c.Defaults[b] != "unsupported" {
		for _, p := range [][2]string{{a, b}, {b, a}} {
			found, err := refused(c, filepath.Join(dir, p[0]), filepath.Join(dir, p[1]))
			keep(false, p[0]+" to "+p[1], found, err)
		}
	}
	if baseDir != "" {
		for _, p := range catalog.Profiles {
			if c.Defaults[p] == "unsupported" {
				continue
			}
			found, err := refused(c, filepath.Join(baseDir, p), filepath.Join(dir, p))
			keep(true, p, found, err)
		}
	}
	return out, errors.Join(errs...)
}

// refused applies each release of c under from, then sends the same release
// under to with --dry-run=server, and returns what the API server refuses
// to update. A release missing under from is new and skipped; one that
// renders the same is too.
func refused(c *catalog.Component, from, to string) ([]catalog.Refusal, error) {
	var found []catalog.Refusal
	var errs []error
	var done applied
	defer func() { removeApplied(done) }()
	for _, rel := range c.Releases {
		before, err := os.ReadFile(filepath.Join(from, c.ID, rel.Name+".yaml"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		after, err := os.ReadFile(filepath.Join(to, c.ID, rel.Name+".yaml"))
		if err != nil {
			return nil, err
		}
		if bytes.Equal(before, after) {
			continue
		}
		docs := releaseDocs(before, rel.Namespace)
		for _, ns := range slices.Sorted(maps.Keys(docs)) {
			done = append(done, switchDoc{ns, docs[ns]})
			if stderr, err := kubectl([]byte(namespaceDoc(ns)), "apply", "-f", "-"); err != nil {
				return nil, fmt.Errorf("namespace %s: %w: %s", ns, err, strings.TrimSpace(stderr))
			}
			if stderr, err := kubectl([]byte(docs[ns]), "apply", "--server-side", "--force-conflicts",
				"--field-manager=catalog", "-n", ns, "-f", "-"); err != nil {
				return nil, fmt.Errorf("apply %s: %w: %s", rel.Name, err, strings.TrimSpace(stderr))
			}
		}
		docs = releaseDocs(after, rel.Namespace)
		for _, ns := range slices.Sorted(maps.Keys(docs)) {
			stderr, err := kubectl([]byte(docs[ns]), "apply", "--server-side", "--force-conflicts", "--dry-run=server",
				"--field-manager=catalog", "-n", ns, "-f", "-")
			if err == nil {
				continue
			}
			x, unread := catalog.ParseRefusals(stderr, rel.Name, ns)
			found = append(found, x...)
			for _, u := range unread {
				errs = append(errs, fmt.Errorf("%s: the API server refused the update: %s", rel.Name, u))
			}
		}
	}
	return found, errors.Join(errs...)
}

func namespaceDoc(ns string) string {
	return fmt.Sprintf("{\"apiVersion\": \"v1\", \"kind\": \"Namespace\", \"metadata\": {\"name\": %q}}\n", ns)
}

// switchDoc is one manifest document and the namespace kubectl applies it in.
type switchDoc struct {
	namespace, doc string
}

// releaseDocs groups a rendered release's objects by namespace. It leaves
// out what an upgrade never replaces: CRDs and Namespaces (the dry run
// installed them), webhook configurations (nothing serves them here), and
// Helm hooks (Helm creates them again on every upgrade).
func releaseDocs(data []byte, defaultNamespace string) map[string]string {
	out := map[string]string{}
	for _, doc := range strings.Split(string(data), "\n---") {
		var o struct {
			Kind     string
			Metadata struct {
				Name, Namespace string
				Annotations     map[string]string
			}
		}
		if yaml.Unmarshal([]byte(doc), &o) != nil || o.Kind == "" {
			continue
		}
		if _, hook := o.Metadata.Annotations["helm.sh/hook"]; hook {
			continue
		}
		switch o.Kind {
		case "CustomResourceDefinition", "Namespace", "ValidatingWebhookConfiguration", "MutatingWebhookConfiguration", "APIService":
			continue
		}
		ns := cmp.Or(o.Metadata.Namespace, defaultNamespace)
		out[ns] += doc + "\n---\n"
	}
	return out
}

// kubectl runs kubectl and returns what it wrote to stderr.
func kubectl(stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}

// applied is what refused created, to delete again.
type applied []switchDoc

// removeApplied deletes the objects, so the next comparison starts empty.
// The API server has no controller to clear finalizers, so it clears them.
func removeApplied(done applied) {
	for _, d := range slices.Backward(done) {
		kubectl([]byte(d.doc), "delete", "--wait=false", "--ignore-not-found", "-n", d.namespace, "-f", "-")
		kubectl([]byte(d.doc), "patch", "--type=merge", "-p", `{"metadata":{"finalizers":null}}`, "-n", d.namespace, "-f", "-")
	}
}
