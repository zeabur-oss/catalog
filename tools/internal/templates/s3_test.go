package templates

import "testing"

func TestSkipPerconaS3VerifyOnlyForE2EEndpoint(t *testing.T) {
	const e2e = "https://s3.templates-e2e-s3.svc.cluster.local"
	cases := []struct {
		name, endpoint string
		skip           bool
	}{
		{name: "e2e-endpoint", endpoint: e2e, skip: true},
		{name: "other-endpoint", endpoint: "https://s3.example.test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := []byte(`apiVersion: psmdb.percona.com/v1
kind: PerconaServerMongoDB
metadata: {name: mongodb}
spec:
  backup:
    storages:
      s3: {type: s3, s3: {bucket: b, endpointUrl: "` + tc.endpoint + `", insecureSkipTLSVerify: false}}
`)
			out, err := skipPerconaS3Verify(manifest, e2e)
			if err != nil {
				t.Fatal(err)
			}
			docs, err := DecodeDocuments(out)
			if err != nil {
				t.Fatal(err)
			}
			s3 := object(object(object(object(object(docs[0]["spec"])["backup"])["storages"])["s3"])["s3"])
			if got := truth(s3["insecureSkipTLSVerify"]); got != tc.skip {
				t.Fatalf("insecureSkipTLSVerify = %v, want %v", got, tc.skip)
			}
		})
	}
}
