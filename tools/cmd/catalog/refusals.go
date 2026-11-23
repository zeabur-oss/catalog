package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// renderBase renders the merge-base of HEAD and the base branch with its own
// tools, into out/render-base. The base branch is $CATALOG_BASE, origin/main
// by default. It returns an empty sha when HEAD is the merge-base or there is
// none, so there is nothing to upgrade from.
func renderBase() (sha, dir string, err error) {
	ref := cmp.Or(os.Getenv("CATALOG_BASE"), "origin/main")
	out, err := catalog.Run(nil, "git", "merge-base", "HEAD", ref)
	if err != nil {
		slog.Warn("no base to upgrade from", "ref", ref, "error", err)
		return "", "", nil
	}
	head, err := catalog.Run(nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", "", err
	}
	sha = strings.TrimSpace(out)
	if sha == strings.TrimSpace(head) {
		return "", "", nil
	}
	src, err := os.MkdirTemp("", "catalog-base-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(src)
	archive := exec.Command("git", "archive", sha)
	extract := exec.Command("tar", "-x", "-C", src)
	if extract.Stdin, err = archive.StdoutPipe(); err != nil {
		return "", "", err
	}
	if err := extract.Start(); err != nil {
		return "", "", err
	}
	if err := errors.Join(archive.Run(), extract.Wait()); err != nil {
		return "", "", fmt.Errorf("git archive %s: %w", sha, err)
	}
	if dir, err = filepath.Abs(filepath.Join("out", "render-base")); err != nil {
		return "", "", err
	}
	slog.Info("rendering the base", "sha", sha)
	cmd := exec.Command("cargo", "run", "--locked", "-p", "zeabur-catalog", "--", "render", "--out", dir)
	if _, err := os.Stat(filepath.Join(src, "tools/cmd/catalog/render.go")); err == nil {
		cmd = exec.Command("go", "run", "./tools/cmd/catalog", "render", "-out", dir)
	}
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("render %s: %w: %s", sha, err, tail(string(out), 20))
	}
	return sha, dir, nil
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

// refusal is a release the API server refused to update, in an upgrade or a
// profile switch, and what kubectl printed.
type refusal struct {
	component, release, when string
	upgrade                  bool // an upgrade from the base, not a profile switch
	output                   string
}

// checkRefusals applies one render to the API server for real, sends the
// other with --dry-run=server, and fails on what the API server refuses: the
// two profiles in both directions, and the base's render of each profile
// replaced by dir's. The dry run step already sent every render to an empty
// API server, so an error here comes from updating one render to the other.
func checkRefusals(r *repo, dir string) error {
	sha, baseDir, err := renderBase()
	if err != nil {
		return err
	}
	if sha != "" {
		m, _ := filepath.Glob(filepath.Join(baseDir, minorVersion(filepath.Base(dir))+".*"))
		if len(m) != 1 {
			return fmt.Errorf("the base renders no Kubernetes %s", minorVersion(filepath.Base(dir)))
		}
		baseDir = m[0]
	}
	var found []refusal
	var errs []error
	for _, c := range r.cat.Components {
		x, err := componentRefusals(c, dir, baseDir)
		found = append(found, x...)
		errs = append(errs, err)
	}
	major := false
	if sha != "" {
		if major, err = addsMajorChangeFile(".", sha); err != nil {
			return err
		}
	}
	return errors.Join(append(errs, judgeRefusals(found, major)...)...)
}

// judgeRefusals returns an error for each refusal. An upgrade refusal passes
// when the pull request adds a major change file, which says how to migrate; a
// profile switch never does.
func judgeRefusals(found []refusal, major bool) []error {
	var errs []error
	for _, x := range found {
		if x.upgrade && major {
			slog.Info("intended break", "component", x.component, "release", x.release, "when", x.when)
			continue
		}
		from, hint := "from "+x.when, "Give the field the same value in both profiles."
		if x.upgrade {
			from, hint = "from the base in "+x.when, "Change the component so the release updates in place, or add a `major` change file with the migration steps."
		}
		errs = append(errs, fmt.Errorf("%s: the API server refuses to update release %s %s. %s\n%s",
			x.component, x.release, from, hint, strings.TrimSpace(x.output)))
	}
	return errs
}

func componentRefusals(c *catalog.Component, dir, baseDir string) ([]refusal, error) {
	var out []refusal
	var errs []error
	compare := func(upgrade bool, when, from, to string) {
		found, err := refused(c, from, to)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", c.ID, when, err))
		}
		for rel, output := range found {
			out = append(out, refusal{c.ID, rel, when, upgrade, output})
		}
	}
	a, b := catalog.Profiles[0], catalog.Profiles[1]
	if c.Defaults[a] != "unsupported" && c.Defaults[b] != "unsupported" {
		compare(false, a+" to "+b, filepath.Join(dir, a), filepath.Join(dir, b))
		compare(false, b+" to "+a, filepath.Join(dir, b), filepath.Join(dir, a))
	}
	if baseDir != "" {
		for _, p := range catalog.Profiles {
			if c.Defaults[p] != "unsupported" {
				compare(true, p, filepath.Join(baseDir, p), filepath.Join(dir, p))
			}
		}
	}
	return out, errors.Join(errs...)
}

// refused applies each release of c under from, then sends the same release
// under to with --dry-run=server, and returns kubectl's output for each
// release the API server refuses to update. A release missing under from is
// new and skipped; one that renders the same is too.
// When the CRDs differ, from's are installed before from's objects and to's
// after them, as in an upgrade. An object that does not apply against its own
// CRDs is an error, not a refusal.
func refused(c *catalog.Component, from, to string) (map[string]string, error) {
	found := map[string]string{}
	var done []nsDocs
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
		oldDocs, err := updatable(before, rel.Namespace)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel.Name, err)
		}
		newDocs, err := updatable(after, rel.Namespace)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel.Name, err)
		}
		oldCRDs, err := crdsOf(before, rel.Namespace)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel.Name, err)
		}
		newCRDs, err := crdsOf(after, rel.Namespace)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel.Name, err)
		}
		changed := oldCRDs.text != newCRDs.text
		if changed {
			if err := oldCRDs.reset(newCRDs.names); err != nil {
				return nil, fmt.Errorf("%s: base CRDs: %w", rel.Name, err)
			}
		}
		for _, d := range oldDocs {
			done = append(done, d)
			if _, err := catalog.Kubectl([]byte(namespaceDoc(d.namespace)), "apply", "-f", "-"); err != nil {
				return nil, fmt.Errorf("namespace %s: %w", d.namespace, err)
			}
			if _, err := catalog.Kubectl([]byte(d.docs), "apply", "--server-side", "--force-conflicts",
				"--field-manager=catalog", "-n", d.namespace, "-f", "-"); err != nil {
				return nil, fmt.Errorf("apply %s: %w", rel.Name, err)
			}
		}
		if changed {
			if err := newCRDs.install(); err != nil {
				return nil, fmt.Errorf("%s: new CRDs: %w", rel.Name, err)
			}
		}
		for _, d := range newDocs {
			if err := dryRunUpdate(d, changed); err != nil {
				var run *catalog.RunError
				if errors.As(err, &run) {
					found[rel.Name] += run.Stderr
				}
			}
		}
	}
	return found, nil
}

// crdSet is the CRDs of a rendered release.
type crdSet struct {
	names []string
	text  string
}

func crdsOf(data []byte, defaultNamespace string) (crdSet, error) {
	docs, err := manifestDocs(data, defaultNamespace)
	if err != nil {
		return crdSet{}, err
	}
	var set crdSet
	for _, d := range docs {
		if d.Kind == "CustomResourceDefinition" {
			set.names = append(set.names, d.Name)
			set.text += d.text + "\n---\n"
		}
	}
	return set, nil
}

// reset deletes the CRDs under their names and under other, with their
// objects, and installs these, so none is left from the version before.
func (s crdSet) reset(other []string) error {
	gone := slices.Concat(s.names, other)
	if len(gone) > 0 {
		if _, err := catalog.Kubectl(nil, append([]string{"delete", "crd", "--ignore-not-found", "--timeout=2m"}, gone...)...); err != nil {
			return err
		}
	}
	return s.install()
}

// install applies the CRDs over the ones in the API server, as an upgrade
// does, and keeps the objects they hold.
func (s crdSet) install() error {
	if len(s.names) == 0 {
		return nil
	}
	if _, err := catalog.Kubectl([]byte(s.text), "apply", "--server-side", "--force-conflicts", "--field-manager=catalog", "-f", "-"); err != nil {
		return err
	}
	_, err := catalog.Kubectl(nil, append([]string{"wait", "--for=condition=Established", "--timeout=2m"}, prefixed("crd/", s.names)...)...)
	return err
}

func prefixed(prefix string, names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = prefix + n
	}
	return out
}

// dryRunUpdate sends the new objects of one namespace over the stored ones.
// The API server serves an updated CRD's schema a moment after the update, so
// with new CRDs a "not declared in schema" error is tried again for a few
// seconds before it counts.
func dryRunUpdate(d nsDocs, newCRDs bool) (err error) {
	for range 10 {
		_, err = catalog.Kubectl([]byte(d.docs), "apply", "--server-side", "--force-conflicts", "--dry-run=server",
			"--field-manager=catalog", "-n", d.namespace, "-f", "-")
		var run *catalog.RunError
		if err == nil || !newCRDs || !errors.As(err, &run) || !strings.Contains(run.Stderr, "not declared in schema") {
			return err
		}
		time.Sleep(time.Second)
	}
	return err
}

func namespaceDoc(ns string) string {
	return fmt.Sprintf("{\"apiVersion\": \"v1\", \"kind\": \"Namespace\", \"metadata\": {\"name\": %q}}\n", ns)
}

// updatable groups a rendered release's objects by namespace, leaving out
// what an upgrade never updates in place: CRDs and Namespaces (the dry run
// step installed them), webhook configurations and APIServices (nothing
// serves them here), and Helm hooks (Helm creates them again on every
// upgrade).
func updatable(data []byte, defaultNamespace string) ([]nsDocs, error) {
	docs, err := manifestDocs(data, defaultNamespace)
	if err != nil {
		return nil, err
	}
	var keep []manifestDoc
	for _, d := range docs {
		switch d.Kind {
		case "CustomResourceDefinition", "Namespace", "ValidatingWebhookConfiguration", "MutatingWebhookConfiguration", "APIService":
			continue
		}
		if !d.hook {
			keep = append(keep, d)
		}
	}
	return byNamespace(keep), nil
}

// removeApplied deletes the objects, so the next comparison starts empty.
// The API server has no controller to clear finalizers, so it clears them.
func removeApplied(done []nsDocs) {
	for _, d := range slices.Backward(done) {
		catalog.Kubectl([]byte(d.docs), "delete", "--wait=false", "--ignore-not-found", "-n", d.namespace, "-f", "-")
		catalog.Kubectl([]byte(d.docs), "patch", "--type=merge", "-p", `{"metadata":{"finalizers":null}}`, "-n", d.namespace, "-f", "-")
	}
}
