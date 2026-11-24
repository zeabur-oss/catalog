package catalog

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
