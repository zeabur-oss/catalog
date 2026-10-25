package templates

import (
	"slices"
	"strings"
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

func TestPerconaClusterChecksBackupsOnlyWhenEnabled(t *testing.T) {
	cases := []struct {
		name   string
		backup map[string]any
		broken bool
	}{
		{name: "off", backup: map[string]any{"enabled": false}},
		{name: "absent"},
		{name: "unverified-tls", backup: map[string]any{"enabled": true, "image": "example.test/pbm@sha256:" + strings.Repeat("a", 64), "storages": map[string]any{"s3": map[string]any{"type": "s3", "s3": map[string]any{"credentialsSecret": "backup-credentials", "endpointUrl": "https://s3.example.test", "insecureSkipTLSVerify": true}}}}, broken: true},
		{name: "unpinned-image", backup: map[string]any{"enabled": true, "image": "example.test/pbm:2", "storages": map[string]any{}}, broken: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Template{Requires: []string{"databases/percona-mongodb"}}
			spec := map[string]any{"image": "example.test/mongod@sha256:" + strings.Repeat("a", 64), "initImage": "example.test/init@sha256:" + strings.Repeat("a", 64)}
			if tc.backup != nil {
				spec["backup"] = tc.backup
			}
			var broken []string
			checkPercona(r, map[string]any{"spec": spec}, nil, "mongodb", func(where string, rule Rule, args ...string) {
				if rule == ErrPerconaBackupStorage || slices.Contains(args, "backup.image") || strings.Contains(where, "/operand-") {
					broken = append(broken, where+": "+string(rule))
				}
			})
			if (len(broken) > 0) != tc.broken {
				t.Fatalf("got %v, want broken=%v", broken, tc.broken)
			}
		})
	}
}
