package templates

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"text/template"
	"time"
)

// S3 is the SeaweedFS S3 gateway that template e2e runs use for backups and
// object storage. Its certificate comes from a test CA.
type S3 struct {
	Endpoint, Host, CIDR, Bucket string
	accessKey, secretKey         string
	ca                           []byte
}

const (
	s3Namespace = "templates-e2e-s3"
	// S3Secret is the Secret in the template namespace with the S3
	// credentials (under each key name the templates use) and ca.crt.
	S3Secret = "e2e-s3"
)

//go:embed s3.yaml
var s3Manifest string

// S3Component is the component that runs SeaweedFS.
const S3Component = "object-storage/seaweedfs-operator"

// startS3 runs SeaweedFS in s3Namespace and returns a function that deletes it.
func startS3() (_ *S3, stop func() error, err error) {
	host := "s3." + s3Namespace + ".svc.cluster.local"
	s := &S3{Endpoint: "https://" + host, Host: host, Bucket: "e2e", accessKey: token(10), secretKey: token(20)}
	cert, key, ca, err := testCertificates(host)
	if err != nil {
		return nil, nil, err
	}
	s.ca = ca
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
		"Namespace": s3Namespace, "Bucket": s.Bucket, "Config": string(config),
		"Cert": base64.StdEncoding.EncodeToString(cert), "Key": base64.StdEncoding.EncodeToString(key),
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
	// The operator's webhook can refuse requests for a short time after install.
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
		{"deployment/e2e-s3", "--for=create", "--timeout=10m"},
		{"deployment/e2e-s3", "--for=condition=Available", "--timeout=10m"},
		{"bucket/e2e", "--for=condition=Ready", "--timeout=5m"},
	} {
		if _, err := kubectl(nil, append([]string{"wait", "-n", s3Namespace}, args...)...); err != nil {
			return nil, nil, err
		}
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
			"ACCESS_KEY_ID": s.accessKey, "ACCESS_SECRET_KEY": s.secretKey,
			"AWS_ACCESS_KEY_ID": s.accessKey, "AWS_SECRET_ACCESS_KEY": s.secretKey,
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

// testCertificates returns a server certificate and key for host, and the CA
// that signed it, all PEM.
func testCertificates(host string) (cert, key, ca []byte, err error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "catalog e2e CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: host}, DNSNames: []string{host},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caTmpl, &srvKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(srvKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvDER}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), nil
}

// skipPerconaS3Verify turns off TLS verification for each Percona MongoDB S3
// storage at endpoint. PBM cannot take a CA, and the e2e CA is disposable.
func skipPerconaS3Verify(manifest []byte, endpoint string) ([]byte, error) {
	docs, err := DecodeDocuments(manifest)
	if err != nil {
		return nil, err
	}
	patched := false
	for _, doc := range docs {
		if text(doc["apiVersion"]) != "psmdb.percona.com/v1" || text(doc["kind"]) != "PerconaServerMongoDB" {
			continue
		}
		for _, storage := range object(object(object(doc["spec"])["backup"])["storages"]) {
			if s3 := object(object(storage)["s3"]); s3 != nil && text(s3["endpointUrl"]) == endpoint {
				s3["insecureSkipTLSVerify"] = true
				patched = true
			}
		}
	}
	if !patched {
		return manifest, nil
	}
	var out bytes.Buffer
	for _, doc := range docs {
		b, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}
		out.WriteString("---\n")
		out.Write(b)
		out.WriteString("\n")
	}
	return out.Bytes(), nil
}
