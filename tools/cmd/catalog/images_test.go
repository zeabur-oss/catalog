package main

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

func TestImagesCheckFailsOnChangedList(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string][]byte{"images/ingress/traefik.yaml": []byte("a\n"), "images/logs/loki.yaml": []byte("b\n")}
	if err := syncImages(files, false, true); err != nil {
		t.Fatal(err)
	}
	if err := syncImages(files, true, true); err != nil {
		t.Errorf("unchanged lists failed the check: %v", err)
	}
	files["images/logs/loki.yaml"] = []byte("c\n")
	err := syncImages(files, true, true)
	if err == nil || !strings.Contains(err.Error(), "images/logs/loki.yaml") || strings.Contains(err.Error(), "traefik") {
		t.Errorf("want an error naming only the changed file, got %v", err)
	}
	if got, _ := os.ReadFile("images/logs/loki.yaml"); string(got) != "b\n" {
		t.Errorf("the check wrote the file: %q", got)
	}
}

func TestImagesCheckFailsOnMissingAndOrphanFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string][]byte{"images/ingress/traefik.yaml": []byte("a\n")}
	if err := syncImages(map[string][]byte{"images/gone/old.yaml": []byte("x\n")}, false, true); err != nil {
		t.Fatal(err)
	}
	err := syncImages(files, true, true)
	if err == nil || !strings.Contains(err.Error(), "traefik.yaml") || !strings.Contains(err.Error(), "old.yaml") {
		t.Errorf("want the missing and the orphan file named, got %v", err)
	}
	if err := syncImages(files, false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("images/gone/old.yaml"); !os.IsNotExist(err) {
		t.Errorf("the orphan file stayed")
	}
}

func TestIndexCarriesImages(t *testing.T) {
	t.Chdir(t.TempDir())
	c := &catalog.Component{ID: "ingress/traefik", Defaults: map[string]string{"single-node": "recommended", "multi-node": "unsupported"}}
	if _, err := readImages(c); err == nil {
		t.Error("a component without a list was accepted")
	}
	data, err := marshalImages(map[string]imageSet{"single-node": {Images: []string{"a@sha256:1"}, UnpinnedImages: []string{"b:2"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := syncImages(map[string][]byte{imagesPath(c.ID): data}, false, false); err != nil {
		t.Fatal(err)
	}
	got, err := readImages(c)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]imageSet{"single-node": {Images: []string{"a@sha256:1"}, UnpinnedImages: []string{"b:2"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
