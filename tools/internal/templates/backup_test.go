package templates

import "testing"

func TestPerconaBackupStaysWithinDeclaredClusterAndStorage(t *testing.T) {
	cases := []struct {
		name, provider, cluster, storage, mode, credentials string
		insecure, plainHTTP, override, reject               bool
	}{
		{name: "declared-logical", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials"},
		{name: "foreign-cluster", provider: "databases/percona-mongodb", cluster: "other", storage: "s3", mode: "logical", credentials: "backup-credentials", reject: true},
		{name: "undeclared-storage", provider: "databases/percona-mongodb", cluster: "database", storage: "other", mode: "logical", credentials: "backup-credentials", reject: true},
		{name: "ambient-identity", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", reject: true},
		{name: "wrong-provider", provider: "databases/cloudnative-pg", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", reject: true},
		{name: "preview-mode", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "incremental", credentials: "backup-credentials", reject: true},
		{name: "unverified-tls", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", insecure: true, reject: true},
		{name: "unencrypted-endpoint", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", plainHTTP: true, reject: true},
		{name: "unsupported-options", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", override: true, reject: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Template{Requires: []string{tc.provider}}
			doc := map[string]any{"spec": map[string]any{"clusterName": tc.cluster, "storageName": tc.storage, "type": tc.mode, "compressionType": "gzip"}}
			if tc.override {
				object(doc["spec"])["backupOptions"] = map[string]any{"args": []any{"--insecure"}}
			}
			endpoint := "https://s3.example.test"
			if tc.plainHTTP {
				endpoint = "http://s3.example.test"
			}
			cluster := map[string]any{"apiVersion": "psmdb.percona.com/v1", "kind": "PerconaServerMongoDB", "metadata": map[string]any{"name": "database"}, "spec": map[string]any{"backup": map[string]any{"enabled": true, "storages": map[string]any{"s3": map[string]any{"type": "s3", "s3": map[string]any{"credentialsSecret": tc.credentials, "endpointUrl": endpoint, "insecureSkipTLSVerify": tc.insecure}}}}}}
			rejected := false
			checkPerconaBackup(r, doc, []map[string]any{cluster}, "backup/initial", func(string, string) { rejected = true })
			if rejected != tc.reject {
				t.Fatalf("rejected = %v, want %v", rejected, tc.reject)
			}
		})
	}
}
