package monitoring

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var separators = regexp.MustCompile(`[^a-z0-9]+`)

func normalize(value string) string {
	decomposed := norm.NFD.String(strings.ToLower(value))
	plain := strings.Map(withoutAccent, decomposed)
	separated := separators.ReplaceAllString(plain, " ")
	return strings.TrimSpace(separated)
}

func withoutAccent(character rune) rune {
	if unicode.Is(unicode.Mn, character) {
		return -1
	}
	return character
}

func containsAny(name string, terms []string) bool {
	matched := false
	for position := 0; position < len(terms) && !matched; position++ {
		matched = containsTerm(name, terms[position])
	}
	return matched
}

func containsTerm(name, term string) bool {
	normalized := normalize(term)
	return normalized != "" && strings.Contains(name, " "+normalized+" ")
}

func (target Target) Matches(name string) bool {
	normalized := " " + normalize(name) + " "
	matched := len(target.RequiredTerms) > 0 && !containsAny(normalized, target.ExcludedTerms)
	for position := 0; position < len(target.RequiredTerms) && matched; position++ {
		matched = containsAny(normalized, target.RequiredTerms[position])
	}
	return matched
}
