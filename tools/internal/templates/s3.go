package templates

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"text/template"
	"time"
)

// S3 is the SeaweedFS S3 gateway that template e2e runs use for object
// storage. cert-manager issues its certificate from a test CA.
type S3 struct {
	Endpoint, CIDR, Bucket string
	accessKey, secretKey   string
	ca                     []byte
}

const (
	s3Namespace = "templates-e2e-s3"
	// S3Secret is the Secret in the template namespace with the S3
	// credentials (access-key and secret-key) and ca.crt.
	S3Secret = "e2e-s3"
)

//go:embed s3.yaml
var s3Manifest string

// S3Component is the component that runs SeaweedFS. It requires cert-manager.
const S3Component = "object-storage/seaweedfs-operator"

// startS3 runs SeaweedFS in s3Namespace and returns a function that deletes it.
func startS3() (_ *S3, stop func() error, err error) {
	host := "s3." + s3Namespace + ".svc.cluster.local"
	s := &S3{Endpoint: "https://" + host, Bucket: "e2e", accessKey: token(10), secretKey: token(20)}
	config, err := json.Marshal(map[string]any{"identities": []any{map[string]any{
		"name":        "e2e",
		"credentials": []any{map[string]string{"accessKey": s.accessKey, "secretKey": s.secretKey}},
		"actions":     []string{"Admin", "Read", "Write", "List", "Tagging"},
	}}})
	if err != nil {
		return nil, nil, err
	}
	var manifest bytes.Buffer
	err = template.Must(template.New("s3").Option("missingkey=error").Parse(s3Manifest)).Execute(&manifest, map[string]string{
		"Namespace": s3Namespace, "Bucket": s.Bucket, "Config": string(config), "Host": host,
	})
	if err != nil {
		return nil, nil, err
	}
	if _, err := kubectl(nil, "create", "namespace", s3Namespace); err != nil {
		return nil, nil, err
	}
	del := func() error {
		_, err := kubectl(nil, "delete", "namespace", s3Namespace, "--wait", "--timeout=5m")
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, del())
		}
	}()
	if _, err := kubectl(nil, "label", "namespace", s3Namespace, "pod-security.kubernetes.io/enforce=restricted"); err != nil {
		return nil, nil, err
	}
	// The webhooks of the operator and cert-manager can refuse requests for a short time after install.
	for start := time.Now(); ; time.Sleep(5 * time.Second) {
		if _, err = kubectl(manifest.Bytes(), "apply", "-f", "-"); err == nil || time.Since(start) > 2*time.Minute {
			break
		}
	}
	if err != nil {
		return nil, nil, err
	}
	// The operator creates the S3 gateway after master, volume and filer.
	for _, args := range [][]string{
		{"certificate/s3-tls", "--for=condition=Ready", "--timeout=5m"},
		{"deployment/e2e-s3", "--for=create", "--timeout=10m"},
		{"deployment/e2e-s3", "--for=condition=Available", "--timeout=10m"},
		{"bucket/e2e", "--for=condition=Ready", "--timeout=5m"},
	} {
		if _, err := kubectl(nil, append([]string{"wait", "-n", s3Namespace}, args...)...); err != nil {
			return nil, nil, err
		}
	}
	ca, err := kubectl(nil, "get", "secret", "s3-tls", "-n", s3Namespace, "-o", `jsonpath={.data.ca\.crt}`)
	if err != nil {
		return nil, nil, err
	}
	if s.ca, err = base64.StdEncoding.DecodeString(strings.TrimSpace(ca)); err != nil {
		return nil, nil, err
	}
	ip, err := kubectl(nil, "get", "pods", "-n", s3Namespace, "-l", "app.kubernetes.io/component=s3,app.kubernetes.io/instance=e2e",
		"--field-selector=status.phase=Running", "-o", "jsonpath={.items[0].status.podIP}")
	if err != nil {
		return nil, nil, err
	}
	s.CIDR = ip + "/32"
	return s, del, nil
}

// secret returns the Secret S3Secret for namespace.
func (s *S3) secret(namespace string) []byte {
	b, _ := json.Marshal(map[string]any{
		"apiVersion": "v1", "kind": "Secret", "type": "Opaque",
		"metadata": map[string]any{"name": S3Secret, "namespace": namespace},
		"stringData": map[string]string{
			"access-key": s.accessKey, "secret-key": s.secretKey,
			"ca.crt": string(s.ca),
		},
	})
	return b
}

func token(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}
