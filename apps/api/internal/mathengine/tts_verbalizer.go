package mathengine

import (
	"regexp"
	"strings"
)

var fracPattern = regexp.MustCompile(`\b(\d+)\s*/\s*(\d+)\b`)

// VerbalizeMath converts mathematical symbols and fractions into natural spoken text
// for English (en) and Indonesian (id) TTS engines.
func VerbalizeMath(text, locale string) string {
	out := text
	if locale == "id" {
		out = fracPattern.ReplaceAllString(out, "$1 per $2")
		replacer := strings.NewReplacer(
			"×", " dikali ",
			"*", " dikali ",
			"÷", " dibagi ",
			"+", " ditambah ",
			"−", " dikurangi ",
			"=", " sama dengan ",
			"%", " persen ",
			"cm²", " sentimeter persegi ",
			"cm³", " sentimeter kubik ",
			"km/h", " kilometer per jam ",
			"km/jam", " kilometer per jam ",
		)
		return collapseSpaces(replacer.Replace(out))
	}

	out = fracPattern.ReplaceAllString(out, "$1 over $2")
	replacer := strings.NewReplacer(
		"×", " times ",
		"*", " times ",
		"÷", " divided by ",
		"+", " plus ",
		"−", " minus ",
		"=", " equals ",
		"%", " percent ",
		"cm²", " square centimeters ",
		"cm³", " cubic centimeters ",
		"km/h", " kilometers per hour ",
	)
	return collapseSpaces(replacer.Replace(out))
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
