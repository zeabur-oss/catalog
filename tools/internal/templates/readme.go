package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// readmeSections are the level-2 headings of a template README, in order, by language ("" is
// English). Every README has the first two.
var readmeSections = map[string][]string{
	"":      {"Introduction", "Getting started", "Configuration", "Troubleshooting"},
	"zh-TW": {"簡介", "開始使用", "設定", "疑難排解"},
	"zh-CN": {"简介", "开始使用", "配置", "故障排除"},
}

// checkReadmes returns a problem for each README of t that is missing, has other level-1 or
// level-2 headings than readmeSections, or whose Getting started does not name the Secret of a
// generated input.
func checkReadmes(t *Template, locales []string) []string {
	var out []string
	for _, lang := range append([]string{""}, locales...) {
		name := catalog.ReadmeFile(lang)
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
		sections, problems := catalog.ReadmeSections(string(data), want, want[:2])
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
