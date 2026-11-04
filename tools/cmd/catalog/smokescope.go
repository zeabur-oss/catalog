package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// toolsChanged reports whether the test tool, or the programs it runs, differ
// between two commits. goTool asks about the tool's binary, nixTools about the
// kubectl, kind and helm of the dev shell.
type toolsChanged func(goTool, nixTools bool) (bool, error)

// scope is what a smoke run without IDs tests.
type scope struct {
	all   bool            // everything
	deps  map[string]bool // changed subjects: they, and what installs them
	exact map[string]bool // only these
}

func (s scope) selects(id string, order []*catalog.Component) bool {
	return s.all || s.exact[id] || s.deps[id] || slices.ContainsFunc(order, func(c *catalog.Component) bool { return s.deps[c.ID] })
}

// smokeScope decides what a smoke run tests after files changed.
//
//   - config/kubernetes.yaml tests everything.
//   - Another config/ file or a workflow tests the representative set.
//   - A change to tools/, go.mod, go.sum or the flake tests the representative
//     set only when check says the tool's binary or its programs changed.
//   - A changed component or template is tested with what installs it.
func smokeScope(r *repo, files []string, check toolsChanged) (scope, error) {
	s := scope{deps: ownIDs(r, files), exact: map[string]bool{}}
	var goFiles, nixFiles, other bool
	for _, f := range files {
		switch {
		case f == "config/kubernetes.yaml":
			s.all = true
		case strings.HasPrefix(f, "tools/"), strings.HasPrefix(f, "go."):
			goFiles = true
		case strings.HasPrefix(f, "flake."):
			nixFiles = true
		case strings.HasPrefix(f, "config/"), strings.HasPrefix(f, ".github/workflows/"):
			other = true
		}
	}
	if s.all {
		return s, nil
	}
	if !other && (goFiles || nixFiles) {
		var err error
		if other, err = check(goFiles, nixFiles); err != nil {
			slog.Warn("cannot compare the test tools; testing the representative set", "err", err)
			other = true
		}
	}
	if other {
		ids, err := representative(r)
		if err != nil {
			return s, err
		}
		for _, id := range ids {
			s.exact[id] = true
		}
	}
	return s, nil
}

// representative returns what smoke tests when the test tool changed: the
// components recommended on single-node and the one template that installs the
// most of them. A tie goes to the smaller ID, so the pick does not move.
func representative(r *repo) ([]string, error) {
	ids := recommended(r.cat, templateProfile)
	var best string
	most := 0
	for _, t := range r.templates {
		order, err := installOrder(r, t.ID, templateProfile)
		if err != nil {
			return nil, err
		}
		n := 0
		for _, c := range order {
			if slices.Contains(ids, c.ID) {
				n++
			}
		}
		if n > most || n == most && n > 0 && t.ID < best {
			best, most = t.ID, n
		}
	}
	if best != "" {
		ids = append(ids, best)
	}
	return ids, nil
}

// smokeSelector is selector for the smoke tier: a change to a shared file
// selects the representative set instead of everything, unless it cannot
// have changed how the tests run.
func smokeSelector(r *repo, since string) (func(string, []*catalog.Component) bool, error) {
	if since == "" || strings.Trim(since, "0") == "" {
		return func(string, []*catalog.Component) bool { return true }, nil
	}
	out, err := catalog.Run(nil, "git", "diff", "--name-only", since, "HEAD")
	if err != nil {
		return nil, err
	}
	s, err := smokeScope(r, strings.Fields(out), func(goTool, nixTools bool) (bool, error) {
		return compareTools(since, goTool, nixTools)
	})
	if err != nil {
		return nil, err
	}
	return s.selects, nil
}

// compareTools builds the tool and evaluates the dev shell at base and at HEAD.
func compareTools(base string, goTool, nixTools bool) (bool, error) {
	dir, err := os.MkdirTemp("", "catalog-tools")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	for name, ref := range map[string]string{"base": base, "head": "HEAD"} {
		if err := exportTree(ref, filepath.Join(dir, name)); err != nil {
			return false, err
		}
	}
	if goTool {
		var sums [2]string
		for i, name := range []string{"base", "head"} {
			if sums[i], err = binaryHash(filepath.Join(dir, name)); err != nil {
				return false, err
			}
		}
		slog.Info("tool binary", "base", sums[0], "head", sums[1])
		if sums[0] != sums[1] {
			return true, nil
		}
	}
	if nixTools {
		var paths [2][]string
		for i, name := range []string{"base", "head"} {
			if paths[i], err = devShellTools(filepath.Join(dir, name)); err != nil {
				return false, err
			}
		}
		slog.Info("dev shell tools", "base", paths[0], "head", paths[1])
		if !slices.Equal(paths[0], paths[1]) {
			return true, nil
		}
	}
	return false, nil
}

// exportTree writes the files of a commit to dir.
func exportTree(ref, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	archive := exec.Command("git", "archive", "--format=tar", ref)
	untar := exec.Command("tar", "-x", "-C", dir)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	untar.Stdin = pipe
	if err := untar.Start(); err != nil {
		return err
	}
	if err := archive.Run(); err != nil {
		return fmt.Errorf("git archive %s: %w", ref, err)
	}
	return untar.Wait()
}

// binaryHash builds the tool in dir without the build's own paths and returns
// the SHA-256 of the binary.
func binaryHash(dir string) (string, error) {
	bin := filepath.Join(dir, "catalog.bin")
	build := exec.Command("go", "build", "-trimpath", "-o", bin, "./tools/cmd/catalog")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build in %s: %w: %s", dir, err, strings.TrimSpace(string(out)))
	}
	f, err := os.Open(bin)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// toolsApply picks the packages of the programs that the e2e runs from the
// dev shell's inputs.
const toolsApply = `ps: map (p: p.outPath) (builtins.filter (p: builtins.any (n: builtins.match (n + "-[0-9].*") p.name != null) ["kubectl" "kind" "kubernetes-helm"]) ps)`

// devShellTools returns the store paths of kubectl, kind and helm in the dev
// shell of the flake in dir.
func devShellTools(dir string) ([]string, error) {
	system := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH] + "-" + runtime.GOOS
	out, err := catalog.Run(nil, "nix", "eval", "--json", "--apply", toolsApply,
		"path:"+dir+"#devShells."+system+".default.nativeBuildInputs")
	if err != nil {
		return nil, err
	}
	var paths []string
	if err := json.Unmarshal([]byte(out), &paths); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no kubectl, kind or helm in the dev shell")
	}
	slices.Sort(paths)
	return paths, nil
}
