package templates

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"go.yaml.in/yaml/v3"
)

type RenderOptions struct {
	Namespace, Domain, ClusterIssuer string
	Inputs                           map[string]string
}

// Errors from Render and GenerateInputs.
var (
	ErrNamespace     = errors.New("namespace must be a DNS label")
	ErrDomain        = errors.New("domain must be a lowercase DNS hostname, not an IP address")
	ErrClusterIssuer = errors.New("cluster issuer must be a DNS subdomain name")
	ErrNoDomain      = errors.New("services with domains need -domain and -cluster-issuer")
	ErrGenerated     = errors.New("is generated into a Secret; render does not take it")
	ErrMissingInput  = errors.New("is required")
	ErrInputPattern  = errors.New("does not match its validation pattern")
)

var namespaceLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
var dnsSubdomain = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$`)

func (o RenderOptions) validate(r *Template) error {
	if !namespaceLabel.MatchString(o.Namespace) || len(o.Namespace) > 63 {
		return ErrNamespace
	}
	if o.Domain != "" && (!dnsSubdomain.MatchString(o.Domain) || len(o.Domain) > 253 || net.ParseIP(o.Domain) != nil) {
		return ErrDomain
	}
	if o.ClusterIssuer != "" && (!dnsSubdomain.MatchString(o.ClusterIssuer) || len(o.ClusterIssuer) > 253) {
		return ErrClusterIssuer
	}
	for _, s := range r.Services {
		if len(s.Domains) > 0 && (o.Domain == "" || o.ClusterIssuer == "") {
			return ErrNoDomain
		}
	}
	patterns, err := validateInputs(r.Inputs)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for i, in := range r.Inputs {
		known[in.Name] = true
		v, ok := o.Inputs[in.Name]
		if in.Generate != "" {
			if ok {
				return fmt.Errorf("input %s %w", in.Name, ErrGenerated)
			}
			continue
		}
		if in.Required && !ok {
			return fmt.Errorf("input %s %w", in.Name, ErrMissingInput)
		}
		if ok && !patterns[i].MatchString(v) {
			return fmt.Errorf("input %s %w", in.Name, ErrInputPattern)
		}
	}
	for k := range o.Inputs {
		if !known[k] {
			return fmt.Errorf("unknown input %s", k)
		}
	}
	return nil
}

// dnsEgress is the NetworkPolicy egress rule to cluster DNS. A manifest inserts it with {{ dnsEgress }}.
const dnsEgress = `{to: [{namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: kube-system}}, podSelector: {matchLabels: {k8s-app: kube-dns}}}], ports: [{protocol: UDP, port: 53}, {protocol: TCP, port: 53}]}`

// Render expands manifests.yaml. Generated inputs stay in their Secrets.
func Render(r *Template, o RenderOptions) ([]byte, error) {
	if err := o.validate(r); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(r.Dir, "manifests.yaml"))
	if err != nil {
		return nil, err
	}
	t, err := parse(data)
	if err != nil {
		return nil, err
	}
	// An optional input that is not given is empty.
	inputs := maps.Clone(o.Inputs)
	if inputs == nil {
		inputs = map[string]string{}
	}
	for _, in := range r.Inputs {
		if _, ok := inputs[in.Name]; !ok && in.Generate == "" {
			inputs[in.Name] = ""
		}
	}
	o.Inputs = inputs
	var out bytes.Buffer
	err = t.Execute(&out, o)
	return out.Bytes(), err
}

// GenerateInputs returns a List of the Secrets that hold generated inputs.
// The output holds credentials.
func GenerateInputs(r *Template, namespace string) ([]byte, error) {
	if !namespaceLabel.MatchString(namespace) || len(namespace) > 63 {
		return nil, ErrNamespace
	}
	patterns, err := validateInputs(r.Inputs)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(r.Inputs))
	for _, in := range r.Inputs {
		if in.Generate != "random" {
			continue
		}
		size := 32
		if in.GenerateBytes != nil {
			size = *in.GenerateBytes
		}
		b := make([]byte, size)
		rand.Read(b)
		if in.GenerateEncoding == "hex" {
			values[in.Name] = hex.EncodeToString(b)
		} else {
			values[in.Name] = base64.RawURLEncoding.EncodeToString(b)
		}
	}
	grouped := map[string]map[string]string{}
	for i, in := range r.Inputs {
		if in.Generate == "" {
			continue
		}
		v := values[in.Name]
		if !patterns[i].MatchString(v) {
			return nil, fmt.Errorf("generated %s %w", in.Name, ErrInputPattern)
		}
		if grouped[in.Secret.Name] == nil {
			grouped[in.Secret.Name] = map[string]string{}
		}
		grouped[in.Secret.Name][in.Secret.Key] = v
	}
	items := []any{}
	for _, n := range slices.Sorted(maps.Keys(grouped)) {
		items = append(items, map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": n, "namespace": namespace}, "type": "Opaque", "stringData": grouped[n]})
	}
	return json.MarshalIndent(map[string]any{"apiVersion": "v1", "kind": "List", "items": items}, "", "  ")
}

// validateInputs checks what the schema cannot: unique names and Secret keys,
// and patterns that compile.
func validateInputs(inputs []Input) ([]*regexp.Regexp, error) {
	byName := map[string]Input{}
	secrets := map[string]bool{}
	patterns := make([]*regexp.Regexp, len(inputs))
	for i, in := range inputs {
		if _, ok := byName[in.Name]; ok {
			return nil, fmt.Errorf("duplicate input %s", in.Name)
		}
		byName[in.Name] = in
		p, err := regexp.Compile(`\A(?:` + in.Validation.Pattern + `)\z`)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", in.Name, err)
		}
		patterns[i] = p
		if in.Secret != nil {
			key := in.Secret.Name + "/" + in.Secret.Key
			if secrets[key] {
				return nil, fmt.Errorf("duplicate Secret key %s", key)
			}
			secrets[key] = true
		}
	}
	return patterns, nil
}

// DecodeDocuments splits a YAML stream into objects and expands Lists.
func DecodeDocuments(b []byte) ([]map[string]any, error) {
	d := yaml.NewDecoder(bytes.NewReader(b))
	var out []map[string]any
	for {
		var v map[string]any
		if err := d.Decode(&v); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if len(v) == 0 {
			continue
		}
		if v["kind"] != "List" {
			out = append(out, v)
			continue
		}
		for _, item := range array(v["items"]) {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("invalid List item")
			}
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no objects")
	}
	return out, nil
}
