package templates

import "testing"

func TestDisablePerconaBackupsOnlyChangesPerconaClusters(t *testing.T) {
	manifest := []byte(`apiVersion: psmdb.percona.com/v1
kind: PerconaServerMongoDB
metadata: {name: mongodb}
spec:
  backup: {enabled: true, storages: {s3: {type: s3}}}
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: db}
spec:
  backup: {enabled: true}
`)
	out, err := disablePerconaBackups(manifest)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := DecodeDocuments(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		enabled := truth(object(object(doc["spec"])["backup"])["enabled"])
		if want := doc["kind"] != "PerconaServerMongoDB"; enabled != want {
			t.Errorf("%s: backup enabled = %v, want %v", doc["kind"], enabled, want)
		}
	}
}
