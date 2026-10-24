package templates_test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestRenderRequiresValidDomainAndIssuer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata: {name: app}\ndata: {host: '{{ .Domain }}'}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &templates.Template{}
	if err := yaml.Unmarshal([]byte(`{"services":[{"name":"app","domains":["{{ .Domain }}"]}]}`), r); err != nil {
		t.Fatal(err)
	}
	r.Dir = dir
	for _, domain := range []string{"", "example.test:443", "example.test#fragment", "example.test\t", "127.0.0.1", "UPPER.test", "-bad.test"} {
		if _, err := templates.Render(r, templates.RenderOptions{Namespace: "test", Domain: domain, ClusterIssuer: "issuer"}); err == nil {
			t.Errorf("invalid domain %q accepted", domain)
		}
	}
	for _, issuer := range []string{"", "issuer:bad", "issuer#bad", "issuer\t"} {
		if _, err := templates.Render(r, templates.RenderOptions{Namespace: "test", Domain: "app.example.test", ClusterIssuer: issuer}); err == nil {
			t.Errorf("invalid issuer %q accepted", issuer)
		}
	}
	if _, err := templates.Render(r, templates.RenderOptions{Namespace: "test", Domain: "app.example.test", ClusterIssuer: "test-issuer"}); err != nil {
		t.Fatal(err)
	}
}
func TestRenderingNeverGeneratesSecretValues(t *testing.T) {
	dir := t.TempDir()
	manifest := "apiVersion: v1\nkind: Pod\nmetadata: {name: app}\nspec:\n  containers:\n  - name: app\n    env:\n    - name: PASSWORD\n      valueFrom: {secretKeyRef: {name: app-secrets, key: password}}\n"
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	r := &templates.Template{}
	if err := yaml.Unmarshal([]byte(`{"services":[{"name":"app"}],"inputs":[{"name":"password","description":"password","generate":"random","secret":{"name":"app-secrets","key":"password"},"validation":{"pattern":"^[A-Za-z0-9_-]{43}$"}}]}`), r); err != nil {
		t.Fatal(err)
	}
	r.Dir = dir
	opts := templates.RenderOptions{Namespace: "test"}
	a, err := templates.Render(r, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := templates.Render(r, opts)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) || strings.Contains(string(a), "stringData") {
		t.Fatal("render generated secret material")
	}
	generated, err := templates.GenerateInputs(r, "test")
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Items []struct {
			StringData map[string]string `json:"stringData"`
		} `json:"items"`
	}
	if err = json.Unmarshal(generated, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || len(list.Items[0].StringData["password"]) != 43 {
		t.Fatal("explicit secret generation did not meet contract")
	}
	opts.Inputs = map[string]string{"password": "do-not-render"}
	if _, err = templates.Render(r, opts); err == nil {
		t.Fatal("generated input override accepted by renderer")
	}
}

func inputRecord(t *testing.T, inputs string) *templates.Template {
	t.Helper()
	r := &templates.Template{}
	if err := yaml.Unmarshal([]byte(`{"inputs":`+inputs+`}`), r); err != nil {
		t.Fatal(err)
	}
	return r
}

func generatedSecrets(t *testing.T, r *templates.Template) map[string]map[string]string {
	t.Helper()
	b, err := templates.GenerateInputs(r, "test")
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			StringData map[string]string `json:"stringData"`
		} `json:"items"`
	}
	if err = json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]map[string]string, len(list.Items))
	last := ""
	for _, item := range list.Items {
		if item.Metadata.Namespace != "test" || item.Metadata.Name <= last {
			t.Fatal("generated Secrets have wrong namespace or nondeterministic name ordering")
		}
		last = item.Metadata.Name
		out[item.Metadata.Name] = item.StringData
	}
	return out
}

func TestRandomInputsMeetComponentByteAndASCIIConstraints(t *testing.T) {
	r := inputRecord(t, `[
		{"name":"realtime","generate":"random","generateBytes":12,"secret":{"name":"z-components","key":"realtime"},"validation":{"pattern":"[A-Za-z0-9_-]{16}"}},
		{"name":"encryption","generate":"random","generateBytes":16,"generateEncoding":"hex","secret":{"name":"a-components","key":"encryption"},"validation":{"pattern":"[0-9a-f]{32}"}},
		{"name":"vault","generate":"random","generateBytes":32,"generateEncoding":"hex","secret":{"name":"a-components","key":"vault"},"validation":{"pattern":"[0-9a-f]{64}"}},
		{"name":"default","generate":"random","secret":{"name":"z-components","key":"default"},"validation":{"pattern":"[A-Za-z0-9_-]{43}"}},
		{"name":"maximum","generate":"random","generateBytes":64,"generateEncoding":"base64url","secret":{"name":"z-components","key":"maximum"},"validation":{"pattern":"[A-Za-z0-9_-]{86}"}}
	]`)
	secrets := generatedSecrets(t, r)
	if len(secrets) != 2 || len(secrets["a-components"]) != 2 || len(secrets["z-components"]) != 3 {
		t.Fatal("inputs were not grouped by Secret name")
	}
	for _, tc := range []struct {
		secret, key  string
		ascii, bytes int
		hex          bool
	}{
		{"z-components", "realtime", 16, 12, false},
		{"a-components", "encryption", 32, 16, true},
		{"a-components", "vault", 64, 32, true},
		{"z-components", "default", 43, 32, false},
		{"z-components", "maximum", 86, 64, false},
	} {
		value := secrets[tc.secret][tc.key]
		if len(value) != tc.ascii {
			t.Fatalf("%s has incorrect ASCII length", tc.key)
		}
		var decoded []byte
		var err error
		if tc.hex {
			decoded, err = hex.DecodeString(value)
		} else {
			decoded, err = base64.RawURLEncoding.DecodeString(value)
		}
		if err != nil || len(decoded) != tc.bytes {
			t.Fatalf("%s has incorrect encoding or random byte count", tc.key)
		}
	}
}

func TestGeneratedValuesRequireFullPatternMatch(t *testing.T) {
	r := inputRecord(t, `[{"name":"seed","generate":"random","secret":{"name":"app","key":"secret"},"validation":{"pattern":"[A-Za-z0-9_-]{42}"}}]`)
	if b, err := templates.GenerateInputs(r, "test"); err == nil || len(b) != 0 {
		t.Fatal("partial random value match accepted")
	}
	r.Inputs[0].Validation.Pattern = "[A-Za-z0-9_-]{42}|[A-Za-z0-9_-]{43}"
	generatedSecrets(t, r)
}

func TestGenerateInputsRejectsInvalidNamespaces(t *testing.T) {
	r := inputRecord(t, `[{"name":"seed","generate":"random","secret":{"name":"app","key":"secret"},"validation":{"pattern":".+"}}]`)
	for _, namespace := range []string{"", "UPPER", "bad/name", "-bad", strings.Repeat("a", 64)} {
		if b, err := templates.GenerateInputs(r, namespace); err == nil || len(b) != 0 {
			t.Fatal("invalid namespace produced credentials")
		}
	}
}

func TestAskInputsRequireFullPatternMatch(t *testing.T) {
	r := inputRecord(t, `[{"name":"ack","required":true,"validation":{"pattern":"yes"}}]`)
	for _, value := range []string{"yes-no", "not-yes", "yes\n"} {
		if b, err := templates.Render(r, templates.RenderOptions{Namespace: "test", Inputs: map[string]string{"ack": value}}); err == nil || len(b) != 0 {
			t.Fatal("partial ask-input match accepted")
		}
	}
}

func TestOptionalInputRendersOnlyWhenGiven(t *testing.T) {
	dir := t.TempDir()
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: app}\ndata:\n{{- if .Inputs.ca }}\n  ca: '{{ .Inputs.ca }}'\n{{- end }}\n  app: x\n"
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	r := inputRecord(t, `[{"name":"ca","required":false,"validation":{"pattern":"[a-z]*"}}]`)
	r.Dir = dir
	for _, tc := range []struct {
		name   string
		inputs map[string]string
		want   bool
	}{
		{"omitted", nil, false},
		{"empty", map[string]string{"ca": ""}, false},
		{"given", map[string]string{"ca": "bundle"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := templates.Render(r, templates.RenderOptions{Namespace: "test", Inputs: tc.inputs})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(out), "ca: 'bundle'"); got != tc.want {
				t.Fatalf("rendered CA = %v, want %v", got, tc.want)
			}
		})
	}
}
