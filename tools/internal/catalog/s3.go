package catalog

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"text/template"
	"time"
)

// S3 is an S3 store for component e2e runs: SeaweedFS in one Pod, with its S3
// API over HTTPS and a certificate from a test CA.
type S3 struct {
	Endpoint, Bucket, AccessKey, SecretKey string
	// CA is the test CA's certificate as PEM.
	CA string
}

const s3Namespace = "catalog-e2e-s3"

//go:embed s3.yaml
var s3Manifest string

// StartS3 runs the store in s3Namespace and returns a function that deletes it.
func StartS3() (_ *S3, stop func() error, err error) {
	host := "s3." + s3Namespace + ".svc"
	s := &S3{Endpoint: "https://" + host, Bucket: "e2e", AccessKey: token(10), SecretKey: token(20)}
	ca, cert, key, err := testCertificates(host)
	if err != nil {
		return nil, nil, err
	}
	s.CA = string(ca)
	config, err := json.Marshal(map[string]any{"identities": []any{map[string]any{
		"name":        "e2e",
		"credentials": []any{map[string]string{"accessKey": s.AccessKey, "secretKey": s.SecretKey}},
		"actions":     []string{"Admin", "Read", "Write", "List", "Tagging"},
	}}})
	if err != nil {
		return nil, nil, err
	}
	certJSON, _ := json.Marshal(string(cert))
	keyJSON, _ := json.Marshal(string(key))
	var manifest bytes.Buffer
	err = template.Must(template.New("s3").Option("missingkey=error").Parse(s3Manifest)).Execute(&manifest, map[string]string{
		"Namespace": s3Namespace, "Config": string(config), "Cert": string(certJSON), "Key": string(keyJSON),
	})
	if err != nil {
		return nil, nil, err
	}
	if _, err := kubectl("create", "namespace", s3Namespace); err != nil {
		return nil, nil, err
	}
	del := func() error { return DeleteNamespace(s3Namespace, manifest.Bytes()) }
	defer func() {
		if err != nil {
			err = errors.Join(err, del())
		}
	}()
	if _, err = Run(manifest.Bytes(), "kubectl", "apply", "-f", "-"); err != nil {
		return nil, nil, err
	}
	if err = (Wait{Resource: "deployment/s3"}).Run(kubectl, s3Namespace, "5m"); err != nil {
		return nil, nil, err
	}
	// The S3 API answers once the master has a volume server, a moment after the Pod is ready.
	err = retry(2*time.Minute, func() error {
		_, err := Run([]byte("s3.bucket.create -name "+s.Bucket), "kubectl", "exec", "-i", "-n", s3Namespace, "deployment/s3", "--", "weed", "shell")
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return s, del, nil
}

// testCertificates returns a CA certificate and a certificate with its key for host, all as PEM.
// They last one day.
func testCertificates(host string) (ca, cert, key []byte, err error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "catalog e2e CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host, host + ".cluster.local"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, caTemplate, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

func token(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}
