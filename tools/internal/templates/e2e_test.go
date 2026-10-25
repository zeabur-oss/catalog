package templates

import "testing"

func TestDisableBackupsRemovesOperatorBackupConfiguration(t *testing.T) {
	manifest := []byte(`apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: {name: db}
spec:
  instances: 1
  backup: {retentionPolicy: 7d}
  plugins: [{name: barman-cloud.cloudnative-pg.io}]
---
apiVersion: postgresql.cnpg.io/v1
kind: Backup
metadata: {name: db-initial}
spec: {cluster: {name: db}}
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata: {name: mariadb-initial-backup}
spec: {mariaDbRef: {name: mariadb}}
---
apiVersion: psmdb.percona.com/v1
kind: PerconaServerMongoDB
metadata: {name: mongodb}
spec:
  backup: {enabled: true}
---
apiVersion: dragonflydb.io/v1alpha1
kind: Dragonfly
metadata: {name: cache}
spec:
  replicas: 1
  snapshot: {dir: s3://e2e/backup}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: backup}
data: {a: b}
`)
	out, off, err := disableBackups(manifest)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := DecodeDocuments(out)
	if err != nil {
		t.Fatal(err)
	}
	kept := map[string]map[string]any{}
	for _, d := range docs {
		kept[text(d["kind"])] = d
	}
	if _, ok := kept["Backup"]; ok {
		t.Error("a Backup object is still in the manifest")
	}
	if _, ok := kept["ConfigMap"]; !ok {
		t.Error("the ConfigMap was removed")
	}
	cluster := object(kept["Cluster"]["spec"])
	if cluster["backup"] != nil || cluster["plugins"] != nil || cluster["instances"] == nil {
		t.Errorf("Cluster spec = %v, want instances only", cluster)
	}
	if truth(object(object(kept["PerconaServerMongoDB"]["spec"])["backup"])["enabled"]) {
		t.Error("Percona backups are still enabled")
	}
	if object(kept["Dragonfly"]["spec"])["snapshot"] != nil {
		t.Error("Dragonfly still has a snapshot")
	}
	for _, c := range []struct {
		resource string
		removed  bool
	}{
		{"backup/db-initial", true},
		{"backups.k8s.mariadb.com/mariadb-initial-backup", true},
		{"backup/other", false},
		{"configmap/backup", false},
	} {
		removed := false
		for _, o := range off {
			removed = removed || o.is(c.resource)
		}
		if removed != c.removed {
			t.Errorf("%s: removed = %v, want %v", c.resource, removed, c.removed)
		}
	}
}

func TestExpandGivesRequiredBackupInputsAMatchingPlaceholder(t *testing.T) {
	r := &Template{ID: "app", Inputs: []Input{
		{Name: "backupDestination", Required: true},
		{Name: "backupEndpoint", Required: true},
		{Name: "backupCidr", Required: true},
		{Name: "backupEndpointPort", Required: true},
		{Name: "backupCASecret"},
		{Name: "adminEmail", Required: true},
	}}
	for i, p := range []string{`s3://[a-z0-9.-]+/[A-Za-z0-9/_-]+`, `[a-zA-Z0-9.-]+:[0-9]{1,5}`, `([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]+`, `[1-9][0-9]{0,4}`, `.*`, `.+@.+`} {
		r.Inputs[i].Validation.Pattern = p
	}
	out, err := expand(r, map[string]string{"backupEndpoint": "given.example.test:9000"}, &S3{})
	if err != nil {
		t.Fatal(err)
	}
	patterns, _ := validateInputs(r.Inputs)
	for i, in := range r.Inputs {
		v, ok := out[in.Name]
		switch in.Name {
		case "backupCASecret", "adminEmail":
			if ok {
				t.Errorf("%s got %q; only required backup inputs get a placeholder", in.Name, v)
			}
		default:
			if !patterns[i].MatchString(v) {
				t.Errorf("%s = %q does not match its pattern", in.Name, v)
			}
		}
	}
	if out["backupEndpoint"] != "given.example.test:9000" {
		t.Errorf("backupEndpoint = %q, want the given value", out["backupEndpoint"])
	}
}
