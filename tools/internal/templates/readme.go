package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// readmeSections are the level-2 headings of a template README, in order, by language ("" is
// English). Every README has the first two.
var readmeSections = map[string][]string{
	"":      {"Introduction", "Getting started", "Configuration", "Troubleshooting"},
	"zh-TW": {"簡介", "開始使用", "設定", "疑難排解"},
	"zh-CN": {"简介", "开始使用", "配置", "故障排除"},
}

// ReadmeFile is the README of a template in lang: README.md in English, else README.<lang>.md.
func ReadmeFile(lang string) string {
	if lang == "" {
		return "README.md"
	}
	return "README." + lang + ".md"
}

// IsReadme reports whether name, a file in a template directory, is one of its READMEs. They are
// docs: they change no install and no measurement.
func IsReadme(name string) bool {
	return strings.HasPrefix(name, "README") && strings.HasSuffix(name, ".md")
}

// checkReadmes returns a problem for each README of t that is missing, has other level-1 or
// level-2 headings than readmeSections, or whose Getting started does not name the Secret of a
// generated input.
func checkReadmes(t *Template, locales []string) []string {
	var out []string
	for _, lang := range append([]string{""}, locales...) {
		name := ReadmeFile(lang)
		want, ok := readmeSections[lang]
		if !ok {
			out = append(out, fmt.Sprintf("%s: no README headings for %s", name, lang))
			continue
		}
		data, err := os.ReadFile(filepath.Join(t.Dir, name))
		if err != nil {
			out = append(out, fmt.Sprintf("%s is missing", name))
			continue
		}
		sections, problems := readmeBody(string(data), want)
		for _, p := range problems {
			out = append(out, name+": "+p)
		}
		for _, in := range t.Inputs {
			if in.Secret != nil && in.Generate != nil && !strings.Contains(sections[want[1]], in.Secret.Name) {
				out = append(out, fmt.Sprintf("%s: %s does not name Secret %s of input %s", name, want[1], in.Secret.Name, in.Name))
			}
		}
	}
	return out
}

// readmeBody returns the text of each level-2 section of a Markdown file, and a problem for each
// heading outside want, out of its order, or missing from the first two of want.
func readmeBody(text string, want []string) (map[string]string, []string) {
	sections := map[string]string{}
	var problems []string
	current, last, fenced := "", -1, false
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		switch {
		case fenced:
		case strings.HasPrefix(line, "# "):
			problems = append(problems, fmt.Sprintf("level-1 heading %q; start with ## %s", line, want[0]))
			continue
		case strings.HasPrefix(line, "## "):
			current = strings.TrimSpace(line[3:])
			i := slices.Index(want, current)
			if i <= last {
				problems = append(problems, fmt.Sprintf("heading %q; the level-2 headings are %s, in that order", line, strings.Join(want, ", ")))
			}
			last = max(last, i)
			sections[current] = ""
			continue
		}
		if current != "" {
			sections[current] += line + "\n"
		}
	}
	for _, h := range want[:2] {
		if _, ok := sections[h]; !ok {
			problems = append(problems, "no ## "+h)
		}
	}
	return sections, problems
}
