package main

import (
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestOnlyIDs(t *testing.T) {
	r := &repo{
		cat:       &catalog.Catalog{Components: []*catalog.Component{{ID: "ingress/traefik"}, {ID: "metrics/metrics-server"}}},
		templates: []*templates.Template{{ID: "ghost"}},
	}
	all, err := onlyIDs(r, "")
	if err != nil || all != nil {
		t.Fatalf("empty list = %v, %v; want every subject", all, err)
	}
	ids, err := onlyIDs(r, "ingress/traefik, ghost")
	if err != nil {
		t.Fatal(err)
	}
	if !ids["ingress/traefik"] || !ids["ghost"] || ids["metrics/metrics-server"] {
		t.Errorf("onlyIDs = %v, want traefik and ghost only", ids)
	}
	if _, err := onlyIDs(r, "ingress/nginx"); err == nil {
		t.Error("an unknown ID is accepted")
	}
}
