package catalog

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaBase is the address of the published schemas, which name each other by relative $ref.
const SchemaBase = "https://oss-catalog.zeabur.com/v1/schema/"

// Locale is a BCP 47 tag: language, then optional script, region, variants,
// extensions and private use, such as zh-TW, zh-Hant-TW or ca-ES-valencia.
type Locale string

// SummaryLocale is a summary in one language.
type SummaryLocale struct {
	Summary string `yaml:"summary" json:"summary"`
}

// SummaryLocales is the summary in other languages, by BCP 47 tag.
// config/locales.yaml lists the tags each needs.
type SummaryLocales map[Locale]SummaryLocale

// CompileSchema compiles the schema file name of dir. A $ref to another
// schema resolves to the file of that name in dir.
func CompileSchema(dir, name string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(data)))
		if err != nil {
			return nil, err
		}
		if err := c.AddResource(SchemaBase+filepath.Base(f), doc); err != nil {
			return nil, err
		}
	}
	return c.Compile(SchemaBase + name)
}
