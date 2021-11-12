package querytranslate

var StemLanguages = []string{
	"english",
	"spanish",
	"french",
	"russian",
	"swedish",
	"norwegian",
}

// languages supported by stopword package
var LanguagesToISOCode = map[string]string{
	"arabic":     "ar",
	"bulgarian":  "bg",
	"czech":      "cs",
	"danish":     "da",
	"english":    "en",
	"finnish":    "fi",
	"french":     "fr",
	"german":     "de",
	"hungarian":  "hu",
	"italian":    "it",
	"japanese":   "ja",
	"khmer":      "km",
	"latvian":    "lv",
	"norwegian":  "no",
	"persian":    "fa",
	"polish":     "pl",
	"portuguese": "pt",
	"romanian":   "ro",
	"russian":    "ru",
	"slovak":     "sk",
	"spanish":    "es",
	"swedish":    "sv",
	"thai":       "th",
	"turkish":    "tr",
}
