package catalog

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestE2EInputsReplaceTestValuesWithTheStore(t *testing.T) {
	c := &Component{ID: "velero", Inputs: []Input{
		{Name: "s3Url", TestValue: "http://minio.invalid:9000"},
		{Name: "region", Default: "us-east-1"},
	}}
	tester := &Tester{Inputs: map[string]map[string]string{"velero": {"s3Url": "{{.S3.Endpoint}}"}}}
	r := newRun()
	r.s3 = &S3{Endpoint: "https://s3.test"}
	got, err := tester.inputs(r, c)
	if err != nil {
		t.Fatal(err)
	}
	if got["s3Url"] != "https://s3.test" || got["region"] != "us-east-1" {
		t.Errorf("inputs = %v", got)
	}
}

func TestTheStoreCertificateVerifiesWithTheTestCA(t *testing.T) {
	host := "s3.catalog-e2e-s3.svc"
	ca, cert, _, err := testCertificates(host)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		t.Fatal("the CA is not PEM")
	}
	block, _ := pem.Decode(cert)
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{host, host + ".cluster.local"} {
		if _, err := leaf.Verify(x509.VerifyOptions{DNSName: name, Roots: roots}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: host}); err == nil {
		t.Error("the certificate verifies without the test CA")
	}
}
