package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// crosscheck writes crosscheck.json: for each template, the options and the
// objects of its e2e render. Zeabur renders the same options with its own
// code and compares the objects.
func crosscheck(fs *flag.FlagSet, r *repo) error {
	out := fs.String("out", "crosscheck.json", "output file")
	fs.Parse(os.Args[2:])
	type entry struct {
		Namespace     string            `json:"namespace"`
		Domain        string            `json:"domain"`
		ClusterIssuer string            `json:"clusterIssuer"`
		Inputs        map[string]string `json:"inputs"`
		Objects       []map[string]any  `json:"objects"`
	}
	all := map[string]entry{}
	for _, t := range r.templates {
		values, err := e2eValues(r.templates, t)
		if err != nil {
			return err
		}
		opts, err := templates.E2EOptions(t, values)
		if err != nil {
			return fmt.Errorf("%s: %w", t.ID, err)
		}
		manifest, err := templates.Render(t, opts)
		if err != nil {
			return fmt.Errorf("%s: %w", t.ID, err)
		}
		objects, err := templates.DecodeDocuments(manifest)
		if err != nil {
			return fmt.Errorf("%s: %w", t.ID, err)
		}
		all[t.ID] = entry{opts.Namespace, opts.Domain, opts.ClusterIssuer, opts.Inputs, objects}
	}
	return writeJSON(*out, all)
}
