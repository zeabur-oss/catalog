package templates

import (
	"slices"
	"testing"
)

func TestPerconaBackupStaysWithinDeclaredClusterAndStorage(t *testing.T) {
	cases := []struct {
		name, provider, cluster, storage, mode, credentials string
		insecure, plainHTTP, override                       bool
		rule                                                Rule
	}{
		{name: "declared-logical", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials"},
		{name: "foreign-cluster", provider: "databases/percona-mongodb", cluster: "other", storage: "s3", mode: "logical", credentials: "backup-credentials", rule: ErrPerconaBackupCluster},
		{name: "undeclared-storage", provider: "databases/percona-mongodb", cluster: "database", storage: "other", mode: "logical", credentials: "backup-credentials", rule: ErrPerconaBackupStorage},
		{name: "ambient-identity", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", rule: ErrPerconaBackupStorage},
		{name: "wrong-provider", provider: "databases/cloudnative-pg", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", rule: ErrPerconaBackupProvider},
		{name: "preview-mode", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "incremental", credentials: "backup-credentials", rule: ErrPerconaBackupMode},
		{name: "unverified-tls", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", insecure: true, rule: ErrPerconaBackupStorage},
		{name: "unencrypted-endpoint", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", plainHTTP: true, rule: ErrPerconaBackupStorage},
		{name: "unsupported-options", provider: "databases/percona-mongodb", cluster: "database", storage: "s3", mode: "logical", credentials: "backup-credentials", override: true, rule: ErrPerconaBackupOverride},
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
			var broken []Rule
			checkPerconaBackup(r, doc, []map[string]any{cluster}, "backup/initial", func(_ string, rule Rule, _ ...string) { broken = append(broken, rule) })
			if (tc.rule == "" && len(broken) > 0) || (tc.rule != "" && !slices.Contains(broken, tc.rule)) {
				t.Fatalf("got %v, want %v", broken, tc.rule)
			}
		})
	}
}
