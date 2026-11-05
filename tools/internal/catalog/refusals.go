package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Refusal is an object the API server refuses to update in place because a
// field cannot change.
type Refusal struct {
	Release   string
	Namespace string
	Kind      string
	Name      string
	// Fields are the field paths the API server names, such as spec.
	Fields []string
}

var (
	invalid = regexp.MustCompile(`(\S+) "([^"]+)" is invalid: (.*)$`)
	cause   = regexp.MustCompile(`(?:^\[|^|, )([\w.\[\]/-]+): (Forbidden|Invalid value|Required value|Unsupported value|Duplicate value|Not found|Too long|Too many|Internal error)`)
)

// ParseRefusals reads kubectl's stderr for objects the API server refused to
// update because a field cannot change. A line it cannot classify that way
// is returned as unread.
func ParseRefusals(output, release, namespace string) (found []Refusal, unread []string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Warning:") {
			continue
		}
		m := invalid.FindStringSubmatch(line)
		if m == nil {
			unread = append(unread, line)
			continue
		}
		fields, ok := immutableFields(m[3])
		if !ok {
			unread = append(unread, line)
			continue
		}
		kind, _, _ := strings.Cut(m[1], ".") // StatefulSet.apps
		found = append(found, Refusal{release, namespace, kind, m[2], fields})
	}
	return found, unread
}

// immutableFields returns the field of each cause, and false unless every
// cause says the field cannot change.
func immutableFields(causes string) ([]string, bool) {
	idx := cause.FindAllStringSubmatchIndex(causes, -1)
	if len(idx) == 0 {
		return nil, false
	}
	var fields []string
	for i, m := range idx {
		end := len(causes)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		kind, detail := causes[m[4]:m[5]], strings.ToLower(causes[m[5]:end])
		if kind != "Forbidden" && !strings.Contains(detail, "immutable") && !strings.Contains(detail, "may not change") {
			return nil, false
		}
		fields = append(fields, causes[m[2]:m[3]])
	}
	return fields, true
}

// Break is an object a change file declares it intends to break, with a
// `Breaks: <component> <Kind> <namespace>/<name>` line.
type Break struct {
	Component, Kind, Namespace, Name string
}

func (b Break) String() string {
	return fmt.Sprintf("%s %s %s/%s", b.Component, b.Kind, b.Namespace, b.Name)
}

// ParseBreaks returns the Breaks: lines of a change file.
func ParseBreaks(text string) ([]Break, error) {
	var out []Break
	for _, line := range strings.Split(text, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Breaks:")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		ns, name, slash := "", "", false
		if len(f) == 3 {
			ns, name, slash = strings.Cut(f[2], "/")
		}
		if !slash || ns == "" || name == "" {
			return nil, fmt.Errorf("%q is not a Breaks: line; write `Breaks: <component> <Kind> <namespace>/<name>`", strings.TrimSpace(line))
		}
		out = append(out, Break{f[0], f[1], ns, name})
	}
	return out, nil
}

// ReadBreaks returns the Breaks: lines of every change file in dir.
func ReadBreaks(dir string) ([]Break, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	var out []Break
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		b, err := ParseBreaks(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out = append(out, b...)
	}
	return out, nil
}
