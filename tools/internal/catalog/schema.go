package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	gen "github.com/invopop/jsonschema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"k8s.io/apimachinery/pkg/api/resource"
)

const module = "github.com/zeabur-oss/catalog"

// SchemaBase is the address of the published schemas, which name each other by relative $ref.
const SchemaBase = "https://oss-catalog.zeabur.com/v1/schema/"

// localePattern is an RFC 5646 langtag in its recommended case.
const localePattern = `^([a-z]{2,3}(-[a-z]{3}){0,3}|[a-z]{4,8})(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?(-([a-z0-9]{5,8}|[0-9][a-z0-9]{3}))*(-[a-wyz0-9](-[a-z0-9]{2,8})+)*(-x(-[a-z0-9]{1,8})+)?$`

// Locale is a BCP 47 tag: language, then optional script, region, variants,
// extensions and private use, such as zh-TW, zh-Hant-TW or ca-ES-valencia.
type Locale string

func (Locale) JSONSchema() *gen.Schema {
	return &gen.Schema{Type: "string", Pattern: localePattern}
}

// SummaryLocale is a summary in one language.
type SummaryLocale struct {
	Summary string `yaml:"summary" json:"summary" jsonschema:"required,pattern=^[^\\n]+$"`
}

// SummaryLocales is the summary in other languages, by BCP 47 tag.
// config/locales.yaml lists the tags each needs.
type SummaryLocales map[Locale]SummaryLocale

func (SummaryLocales) JSONSchemaExtend(s *gen.Schema) { LocaleKeys(s) }

// LocaleKeys restricts the keys of a map schema to BCP 47 tags.
func LocaleKeys(s *gen.Schema) { s.PropertyNames = Locale("").JSONSchema() }

// typed is an object whose type is t.
func typed(t string) *gen.Schema {
	p := gen.NewProperties()
	p.Set("type", &gen.Schema{Const: t})
	return &gen.Schema{Properties: p}
}

// anyRequired holds when an object has any of fields.
func anyRequired(fields ...string) *gen.Schema {
	s := &gen.Schema{}
	for _, f := range fields {
		s.AnyOf = append(s.AnyOf, &gen.Schema{Required: []string{f}})
	}
	return s
}

// NewReflector returns a reflector for the files whose field names are in
// the struct tag fieldTag. Descriptions are the doc comments of the Go
// source under the working directory, which must be the repository root.
func NewReflector(fieldTag string) (*gen.Reflector, error) {
	r := &gen.Reflector{
		FieldNameTag:               fieldTag,
		RequiredFromJSONSchemaTags: fieldTag != "json",
		ExpandedStruct:             true,
		Namer: func(t reflect.Type) string {
			if t.Name() == "" {
				return ""
			}
			r, n := utf8.DecodeRuneInString(t.Name())
			return string(unicode.ToLower(r)) + t.Name()[n:]
		},
		Mapper: func(t reflect.Type) *gen.Schema {
			if t == reflect.TypeFor[resource.Quantity]() {
				return &gen.Schema{Type: "string", Pattern: quantityPattern,
					Description: "A Kubernetes quantity, such as 256Mi or 100m."}
			}
			return nil
		},
	}
	for _, dir := range []string{"tools/internal", "tools/cmd"} {
		if err := r.AddGoComments(module, dir, gen.WithFullComment()); err != nil {
			return nil, err
		}
	}
	// reflect gives the types of the catalog command the package path "main".
	r.LookupComment = func(t reflect.Type, field string) string {
		if t.PkgPath() != "main" {
			return ""
		}
		return r.CommentMap[strings.TrimSuffix(module+"/tools/cmd/catalog."+t.Name()+"."+field, ".")]
	}
	return r, nil
}

// quantityPattern is the serialized form of a Kubernetes quantity.
const quantityPattern = `^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)(([KMGTPE]i)|[numkMGTPE]|([eE][+-]?[0-9]+))?$`

// Reflect returns the schema of v, published as name.
func Reflect(r *gen.Reflector, v any, name, title string) ([]byte, error) {
	s := r.Reflect(v)
	s.ID = gen.ID(SchemaBase + name)
	s.Title = title
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

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
