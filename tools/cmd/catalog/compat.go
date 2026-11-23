package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// compat fails unless every file of the API in <api> passes the schemas of the published API in
// <published>, such as the latest release's v1/, and its own. A reader of v1 checks the data, not
// the text of the schemas, so the data is what must keep passing.
func compat(fs *flag.FlagSet, _ *repo) error {
	fs.Parse(os.Args[2:])
	if fs.NArg() != 2 {
		return errors.New("usage: catalog compat <dir of a published v1> <dir of the new v1>")
	}
	return checkCompat(fs.Arg(0), fs.Arg(1))
}

// checkCompat checks the API in api against the schemas of published, then against its own.
func checkCompat(published, api string) error {
	if err := checkAPI(api, filepath.Join(published, "schema")); err != nil {
		return fmt.Errorf("a change inside v1 must keep passing the published schemas; a breaking change goes to v2:\n%w", err)
	}
	return checkAPI(api, filepath.Join(api, "schema"))
}
