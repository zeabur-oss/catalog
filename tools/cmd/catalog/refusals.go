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
	"k8s.io/apimachinery/pkg/util/version"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// previousRender is the previous release's render, made in the background.
type previousRender struct {
	version string // empty when there is no previous release
	dir     string
	done    chan error
}

// startPreviousRender renders the previous release with its own tools, the
// way that release rendered itself.
func startPreviousRender() *previousRender {
	p := &previousRender{done: make(chan error, 1)}
	tag, err := previousRelease()
	if err != nil || tag == "" {
		if tag == "" && err == nil {
			slog.Info("no previous release tag; no upgrade to dry-run")
		}
		p.done <- err
		return p
	}
	p.version = strings.TrimPrefix(tag, "v")
	go func() { p.done <- p.render(tag) }()
	return p
}

func (p *previousRender) render(tag string) error {
	src, err := os.MkdirTemp("", "catalog-"+tag+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(src)
	archive := exec.Command("git", "archive", tag)
	extract := exec.Command("tar", "-x", "-C", src)
	extract.Stdin, err = archive.StdoutPipe()
	if err != nil {
		return err
	}
	if err := extract.Start(); err != nil {
		return err
	}
	if err := errors.Join(archive.Run(), extract.Wait()); err != nil {
		return fmt.Errorf("git archive %s: %w", tag, err)
	}
	p.dir, err = filepath.Abs(filepath.Join("out", "render-"+tag))
	if err != nil {
		return err
	}
	slog.Info("rendering the previous release", "tag", tag)
	cmd := exec.Command("go", "run", "./tools/cmd/catalog", "render", "-out", p.dir)
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("render %s: %w: %s", tag, err, tail(string(out), 20))
	}
	return nil
}

// wait returns the previous render's directory for the Kubernetes version
// with the same minor as kv, or "" when there is no previous release.
func (p *previousRender) wait(kv string) (string, error) {
	if err := <-p.done; err != nil || p.version == "" {
		return "", err
	}
	m, _ := filepath.Glob(filepath.Join(p.dir, minorVersion(kv)+".*"))
	if len(m) != 1 {
		return "", fmt.Errorf("release %s rendered no Kubernetes %s", p.version, minorVersion(kv))
	}
	return m[0], nil
}

// previousRelease returns the newest release tag in HEAD's history, other
// than one on HEAD itself: the release that the next one upgrades from.
func previousRelease() (string, error) {
	merged, err := catalog.Run(nil, "git", "tag", "--merged", "HEAD", "--list", "v[0-9]*")
	if err != nil {
		return "", err
	}
	here, err := catalog.Run(nil, "git", "tag", "--points-at", "HEAD")
	if err != nil {
		return "", err
	}
	var best string
	var bestVersion *version.Version
	for _, tag := range strings.Fields(merged) {
		if slices.Contains(strings.Fields(here), tag) {
			continue
		}
		v, err := version.ParseSemantic(strings.TrimPrefix(tag, "v"))
		if err != nil {
			continue
		}
		if bestVersion == nil || bestVersion.LessThan(v) {
			best, bestVersion = tag, v
		}
	}
	return best, nil
}

func minorVersion(v string) string { return strings.Join(strings.SplitN(v, ".", 3)[:2], ".") }

// refusal is an object the API server refused to update, and in which
// upgrade or profile switch.
type refusal struct {
	component string
	when      string
	upgrade   bool // an upgrade from the previous release, not a profile switch
	catalog.Refusal
}

// checkRefusals applies one render to the API server for real, sends the
// other with --dry-run=server, and fails on what the API server refuses: the
// two profiles in both directions, and the previous release's render of each
// profile replaced by dir's.
func checkRefusals(r *repo, dir string, prev *previousRender) error {
	prevDir, err := prev.wait(filepath.Base(dir))
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
			found[i], errs[i] = componentRefusals(c, dir, prevDir, prev.version)
		})
	}
	wg.Wait()
	breaks, err := catalog.ReadBreaks(".changeset")
	if err != nil {
		return err
	}
	return errors.Join(append(errs, judgeRefusals(slices.Concat(found...), breaks)...)...)
}

// judgeRefusals returns an error for each refusal that no Breaks: line
// declares, and for each Breaks: line that no upgrade refusal matches. A
// declared upgrade refusal is logged as intended. A profile switch is never
// declared.
func judgeRefusals(found []refusal, breaks []catalog.Break) []error {
	var errs []error
	used := make([]bool, len(breaks))
	for _, x := range found {
		if x.upgrade {
			i := slices.IndexFunc(breaks, func(b catalog.Break) bool {
				return b == catalog.Break{Component: x.component, Kind: x.Kind, Namespace: x.Namespace, Name: x.Name}
			})
			if i >= 0 {
				used[i] = true
				slog.Info("intended break", "component", x.component, "object", breaks[i].String(), "when", x.when)
				continue
			}
		}
		errs = append(errs, fmt.Errorf("%s: %s, the API server refuses to update %s %s/%s (%s); "+
			"keep these fields the same, since Kubernetes can't change them in place. "+
			"If an upgrade has to break it, add a `Breaks: %s %s %s/%s` line and the migration steps to the change file in .changeset/",
			x.component, x.when, x.Kind, x.Namespace, x.Name, strings.Join(x.Fields, ", "),
			x.component, x.Kind, x.Namespace, x.Name))
	}
	for i, b := range breaks {
		if !used[i] {
			errs = append(errs, fmt.Errorf("the change file says `Breaks: %s`, but the API server refused nothing there when upgrading from the previous release; remove the line", b))
		}
	}
	return errs
}

func componentRefusals(c *catalog.Component, dir, prevDir, prevVersion string) ([]refusal, error) {
	var out []refusal
	var errs []error
	keep := func(when string, found []catalog.Refusal, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", c.ID, when, err))
		}
		for _, x := range found {
			out = append(out, refusal{c.ID, when, strings.HasPrefix(when, "upgrading"), x})
		}
	}
	a, b := catalog.Profiles[0], catalog.Profiles[1]
	if c.Defaults[a] != "unsupported" && c.Defaults[b] != "unsupported" {
		for _, p := range [][2]string{{a, b}, {b, a}} {
			found, err := refused(c, filepath.Join(dir, p[0]), filepath.Join(dir, p[1]))
			keep("switching "+p[0]+" to "+p[1], found, err)
		}
	}
	if prevDir != "" {
		for _, p := range catalog.Profiles {
			if c.Defaults[p] == "unsupported" {
				continue
			}
			found, err := refused(c, filepath.Join(prevDir, p), filepath.Join(dir, p))
			keep("upgrading "+p+" from "+prevVersion, found, err)
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
