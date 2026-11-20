package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// componentSections are the level-2 headings of a component README, in order, by language ("" is
// English). Before you install and Troubleshooting are optional.
var componentSections = map[string][]string{
	"":      {"Introduction", "Before you install", "Use it", "Remove it", "Troubleshooting"},
	"zh-TW": {"簡介", "安裝前準備", "使用方式", "移除", "疑難排解"},
	"zh-CN": {"简介", "安装前准备", "使用方法", "移除", "故障排除"},
}

// ReadmePath is the README of component c in lang: components/<id>.README.md in English, else
// components/<id>.README.<lang>.md.
func (c *Component) ReadmePath(lang string) string {
	if lang == "" {
		return "components/" + c.ID + ".README.md"
	}
	return "components/" + c.ID + ".README." + lang + ".md"
}

// needsSetup reports whether an input of c must get a value from outside: it is required and has
// no default and no generated value.
func (c *Component) needsSetup() bool {
	return slices.ContainsFunc(c.Inputs, func(in Input) bool {
		return in.Required && in.Default == nil && in.Generate == nil
	})
}

// readmeProblems returns a problem for each README of c that is missing or has other headings
// than componentSections. Before you install is required when an input needs a value from outside.
func (c *Component) readmeProblems(root string, locales []string) []string {
	var out []string
	for _, lang := range append([]string{""}, locales...) {
		name := c.ReadmePath(lang)
		want, ok := componentSections[lang]
		if !ok {
			out = append(out, fmt.Sprintf("%s: no README headings for %s", name, lang))
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			out = append(out, name+" is missing")
			continue
		}
		required := []string{want[0], want[2], want[3]}
		if c.needsSetup() {
			required = append(required, want[1])
		}
		_, problems := ReadmeSections(string(data), want, required)
		for _, p := range problems {
			out = append(out, name+": "+p)
		}
	}
	return out
}

// ReadmeSections returns the text of each level-2 section of a Markdown file, and a problem for
// each level-1 heading, each level-2 heading outside want or out of its order, and each heading
// of required that is missing.
func ReadmeSections(text string, want, required []string) (map[string]string, []string) {
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
	for _, h := range required {
		if _, ok := sections[h]; !ok {
			problems = append(problems, "no ## "+h)
		}
	}
	return sections, problems
}
