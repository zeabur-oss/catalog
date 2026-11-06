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
	cmd := exec.Command("go", "run", "./tools/cmd/catalog", "render", "-out", dir)
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
		verb, hint := "switch", "Give the field the same value in both profiles."
		if x.upgrade {
			verb, hint = "upgrade", "Change the component so the object updates in place, or add a `major` change file with the migration steps."
		}
		errs = append(errs, fmt.Errorf("%s: the API server refuses to %s release %s in place for %s. %s\n%s",
			x.component, verb, x.release, x.when, hint, strings.TrimSpace(x.output)))
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
		for _, d := range updatable(before, rel.Namespace) {
			done = append(done, d)
			if stderr, err := kubectl([]byte(namespaceDoc(d.namespace)), "apply", "-f", "-"); err != nil {
				return nil, fmt.Errorf("namespace %s: %w: %s", d.namespace, err, strings.TrimSpace(stderr))
			}
			if stderr, err := kubectl([]byte(d.docs), "apply", "--server-side", "--force-conflicts",
				"--field-manager=catalog", "-n", d.namespace, "-f", "-"); err != nil {
				return nil, fmt.Errorf("apply %s: %w: %s", rel.Name, err, strings.TrimSpace(stderr))
			}
		}
		for _, d := range updatable(after, rel.Namespace) {
			if stderr, err := kubectl([]byte(d.docs), "apply", "--server-side", "--force-conflicts", "--dry-run=server",
				"--field-manager=catalog", "-n", d.namespace, "-f", "-"); err != nil {
				found[rel.Name] += stderr
			}
		}
	}
	return found, nil
}

func namespaceDoc(ns string) string {
	return fmt.Sprintf("{\"apiVersion\": \"v1\", \"kind\": \"Namespace\", \"metadata\": {\"name\": %q}}\n", ns)
}

// updatable groups a rendered release's objects by namespace, leaving out
// what an upgrade never updates in place: CRDs and Namespaces (the dry run
// step installed them), webhook configurations and APIServices (nothing
// serves them here), and Helm hooks (Helm creates them again on every
// upgrade).
func updatable(data []byte, defaultNamespace string) []nsDocs {
	var keep []manifestDoc
	for _, d := range manifestDocs(data, defaultNamespace) {
		switch d.Kind {
		case "CustomResourceDefinition", "Namespace", "ValidatingWebhookConfiguration", "MutatingWebhookConfiguration", "APIService":
			continue
		}
		if !d.hook {
			keep = append(keep, d)
		}
	}
	return byNamespace(keep)
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

// removeApplied deletes the objects, so the next comparison starts empty.
// The API server has no controller to clear finalizers, so it clears them.
func removeApplied(done []nsDocs) {
	for _, d := range slices.Backward(done) {
		kubectl([]byte(d.docs), "delete", "--wait=false", "--ignore-not-found", "-n", d.namespace, "-f", "-")
		kubectl([]byte(d.docs), "patch", "--type=merge", "-p", `{"metadata":{"finalizers":null}}`, "-n", d.namespace, "-f", "-")
	}
}
