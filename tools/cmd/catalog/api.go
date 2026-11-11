package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gen "github.com/invopop/jsonschema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// apiMajor is the N of the /vN/ path of the API. It changes only on an incompatible change.
const apiMajor = 1

// apiSchemas are the schemas that the API publishes under schema/.
var apiSchemas = []string{"catalog.schema.json", "component-file.schema.json", "component.schema.json"}

// CatalogFile is catalog.json: the catalog release, its test environments and its components.
type CatalogFile struct {
	Catalog CatalogRelease `json:"catalog"`
	// The profiles that every component describes.
	Profiles []string `json:"profiles"`
	// The Kubernetes versions that every profile is rendered and schema-checked against.
	Kubernetes []string `json:"kubernetes"`
	// The release of each host distribution that e2e installs. A measurement at another
	// distributionVersion tested another release.
	Distributions []PinnedDistribution `json:"distributions"`
	Components    []ComponentEntry     `json:"components"`
}

// CatalogRelease is the catalog release that the API was built from.
type CatalogRelease struct {
	// The catalog version, such as 0.5.0.
	Version string `json:"version" jsonschema:"minLength=1"`
	// The Git commit of the catalog.
	Commit string `json:"commit" jsonschema:"pattern=^[0-9a-f]{40}$"`
}

// PinnedDistribution is the release of a distribution that e2e installs on the host.
type PinnedDistribution struct {
	// k3s, rke2 or kubesolo. A reader ignores a value it does not know.
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ComponentEntry names a component. components/<id>.json holds the rest.
type ComponentEntry struct {
	// The file name of the component without .yaml.
	ID string `json:"id" jsonschema:"pattern=^[a-z0-9]+(-[a-z0-9]+)*$"`
	// The Go module hash (h1:) of components/<id>.yaml. A measurement with another sourceDigest
	// tested an older file.
	SourceDigest string `json:"sourceDigest"`
}

// ComponentFile is components/<id>.json: the declaration of a component, the objects it renders
// and the runs of its e2e test.
type ComponentFile struct {
	ComponentEntry
	// components/<id>.yaml as written.
	Declaration Declaration `json:"declaration"`
	Render      Render      `json:"render"`
	// Every recorded run of the component's e2e test, passed or failed.
	Measurements []Environment `json:"measurements"`
}

// Declaration is a component file, decoded as written.
type Declaration struct{ raw any }

func (d Declaration) MarshalJSON() ([]byte, error) { return json.Marshal(d.raw) }

func (Declaration) JSONSchema() *gen.Schema { return &gen.Schema{Ref: "component.schema.json"} }

// Render is what helm template renders for each profile with the test inputs and the values that
// other components contribute.
type Render struct {
	// The Kubernetes version of the render: the newest in catalog.json kubernetes.
	KubernetesVersion string `json:"kubernetesVersion"`
	// The objects of each profile. A profile where the component is unsupported is missing.
	Profiles map[string]RenderedProfile `json:"profiles"`
}

// RenderedProfile is the objects of a component on one profile.
type RenderedProfile struct {
	Workloads              []RenderedWorkload `json:"workloads"`
	PersistentVolumeClaims []RenderedClaim    `json:"persistentVolumeClaims"`
}

// RenderedWorkload is a Deployment, StatefulSet or DaemonSet. A workload that an operator creates
// later is missing.
type RenderedWorkload struct {
	// The release of the component that renders it.
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Deployment, StatefulSet or DaemonSet. A reader ignores a value it does not know.
	Kind string `json:"kind"`
	// spec.replicas, 1 if not set. A DaemonSet has none: it runs one pod on each node.
	Replicas       *int32              `json:"replicas,omitempty"`
	Containers     []RenderedContainer `json:"containers"`
	InitContainers []RenderedContainer `json:"initContainers,omitempty"`
	// The claims that a StatefulSet makes for each pod.
	VolumeClaimTemplates []RenderedClaim `json:"volumeClaimTemplates,omitempty"`
}

// RenderedContainer is the requests and limits of a container, by resource name.
type RenderedContainer struct {
	Name     string                       `json:"name"`
	Requests map[string]resource.Quantity `json:"requests,omitempty"`
	Limits   map[string]resource.Quantity `json:"limits,omitempty"`
}

// RenderedClaim is the requested storage of a PersistentVolumeClaim.
type RenderedClaim struct {
	// The release of the component that renders it. A volumeClaimTemplates entry has none.
	Release string `json:"release,omitempty"`
	// The namespace of a PersistentVolumeClaim. A volumeClaimTemplates entry has none.
	Namespace string            `json:"namespace,omitempty"`
	Name      string            `json:"name"`
	Storage   resource.Quantity `json:"storage"`
}

// api writes the static API to <out>/v<apiMajor>/ and checks it against its schemas.
func api(fs *flag.FlagSet, r *repo) error {
	version := fs.String("version", "", "catalog release version, such as 0.5.0")
	out := fs.String("out", "out/api", "output directory")
	fs.Parse(os.Args[2:])
	if *version == "" {
		return errors.New("-version is required")
	}
	commit, err := catalog.Run(nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	dir := filepath.Join(*out, fmt.Sprintf("v%d", apiMajor))
	if err := writeAPI(r.cat, cfg, dir, *version, strings.TrimSpace(commit)); err != nil {
		return err
	}
	return checkAPI(dir)
}

// writeAPI replaces dir with catalog.json, components/<id>.json and the schemas of schema/v1.
func writeAPI(cat *catalog.Catalog, cfg config, dir, version, commit string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	kube := cfg.Versions[len(cfg.Versions)-1]
	index := CatalogFile{
		Catalog:    CatalogRelease{Version: version, Commit: commit},
		Profiles:   catalog.Profiles,
		Kubernetes: cfg.Versions,
		Components: []ComponentEntry{},
	}
	for _, p := range hostProfiles {
		index.Distributions = append(index.Distributions, PinnedDistribution{Name: p, Version: cfg.pinnedVersion(p)})
	}
	for _, c := range cat.Components {
		digest, err := sourceDigest(c.Path)
		if err != nil {
			return err
		}
		entry := ComponentEntry{ID: c.ID, SourceDigest: digest}
		index.Components = append(index.Components, entry)
		rendered, err := renderFacts(cat, c, kube)
		if err != nil {
			return fmt.Errorf("%s: %w", c.ID, err)
		}
		file := ComponentFile{
			ComponentEntry: entry,
			Declaration:    Declaration{cat.Raw[c.Path]},
			Render:         rendered,
			Measurements:   readMeasurement(measurementPath(c.ID)).Environments,
		}
		if file.Measurements == nil {
			file.Measurements = []Environment{}
		}
		if err := writeJSON(filepath.Join(dir, "components", c.ID+".json"), file); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(dir, "catalog.json"), index); err != nil {
		return err
	}
	for _, name := range apiSchemas {
		data, err := os.ReadFile(filepath.Join("schema", "v1", name))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dir, "schema"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "schema", name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// checkAPI fails unless each file of the API in dir passes its schema.
func checkAPI(dir string) error {
	var errs []error
	check := func(schema, file string) {
		s, err := catalog.CompileSchema(filepath.Join(dir, "schema"), schema)
		if err != nil {
			errs = append(errs, err)
			return
		}
		f, err := os.Open(file)
		if err != nil {
			errs = append(errs, err)
			return
		}
		defer f.Close()
		doc, err := jsonschema.UnmarshalJSON(f)
		if err == nil {
			err = s.Validate(doc)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
		}
	}
	check("catalog.schema.json", filepath.Join(dir, "catalog.json"))
	files, err := filepath.Glob(filepath.Join(dir, "components", "*.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		check("component-file.schema.json", f)
	}
	return errors.Join(errs...)
}

// renderFacts renders every release of c on each profile it supports and reads its workloads and
// PersistentVolumeClaims.
func renderFacts(cat *catalog.Catalog, c *catalog.Component, kubeVersion string) (Render, error) {
	out := Render{KubernetesVersion: kubeVersion, Profiles: map[string]RenderedProfile{}}
	for _, p := range catalog.Profiles {
		if c.Defaults[p] == "unsupported" {
			continue
		}
		profile := RenderedProfile{Workloads: []RenderedWorkload{}, PersistentVolumeClaims: []RenderedClaim{}}
		for _, rel := range c.Releases {
			extra, err := contributions(cat, c.ID, rel.Name)
			if err != nil {
				return Render{}, err
			}
			manifest, err := catalog.Template(c, rel, p, kubeVersion, extra...)
			if err != nil {
				return Render{}, fmt.Errorf("%s %s: %w", rel.Name, p, err)
			}
			if err := readObjects(manifest, rel, &profile); err != nil {
				return Render{}, fmt.Errorf("%s %s: %w", rel.Name, p, err)
			}
		}
		out.Profiles[p] = profile
	}
	return out, nil
}

// readObjects adds the workloads and PersistentVolumeClaims of manifest, the objects of rel, to
// profile.
func readObjects(manifest []byte, rel catalog.Release, profile *RenderedProfile) error {
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	for {
		var obj struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				Replicas             *int32                            `json:"replicas"`
				Template             corev1.PodTemplateSpec            `json:"template"`
				VolumeClaimTemplates []corev1.PersistentVolumeClaim    `json:"volumeClaimTemplates"`
				Resources            corev1.VolumeResourceRequirements `json:"resources"`
			} `json:"spec"`
		}
		if err := dec.Decode(&obj); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		ns := obj.Metadata.Namespace
		if ns == "" {
			ns = rel.Namespace
		}
		switch obj.Kind {
		case "Deployment", "StatefulSet", "DaemonSet":
			w := RenderedWorkload{Release: rel.Name, Namespace: ns, Name: obj.Metadata.Name, Kind: obj.Kind,
				Containers: containers(obj.Spec.Template.Spec.Containers), InitContainers: containers(obj.Spec.Template.Spec.InitContainers)}
			if obj.Kind != "DaemonSet" {
				w.Replicas = new(int32(1))
				if obj.Spec.Replicas != nil {
					w.Replicas = obj.Spec.Replicas
				}
			}
			if obj.Kind == "StatefulSet" {
				for _, t := range obj.Spec.VolumeClaimTemplates {
					w.VolumeClaimTemplates = append(w.VolumeClaimTemplates,
						RenderedClaim{Name: t.Name, Storage: t.Spec.Resources.Requests[corev1.ResourceStorage]})
				}
			}
			profile.Workloads = append(profile.Workloads, w)
		case "PersistentVolumeClaim":
			profile.PersistentVolumeClaims = append(profile.PersistentVolumeClaims, RenderedClaim{
				Release: rel.Name, Namespace: ns, Name: obj.Metadata.Name, Storage: obj.Spec.Resources.Requests[corev1.ResourceStorage]})
		}
	}
}

// containers returns the requests and limits of each container in list.
func containers(list []corev1.Container) []RenderedContainer {
	out := []RenderedContainer{}
	for _, c := range list {
		out = append(out, RenderedContainer{Name: c.Name, Requests: quantities(c.Resources.Requests), Limits: quantities(c.Resources.Limits)})
	}
	return out
}

func quantities(l corev1.ResourceList) map[string]resource.Quantity {
	if len(l) == 0 {
		return nil
	}
	out := map[string]resource.Quantity{}
	for name, q := range l {
		out[string(name)] = q
	}
	return out
}
