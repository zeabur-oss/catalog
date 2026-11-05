package catalog

import (
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
