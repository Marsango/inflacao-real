package catalog

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func normalizedGTIN(value string) string { return strings.Repeat("0", 14-len(value)) + value }

type textNormalizer struct {
	output   strings.Builder
	previous rune
}

func normalizedText(value string) string {
	var normalizer textNormalizer
	for _, character := range norm.NFD.String(strings.ToLower(value)) {
		normalizer.append(character)
	}
	return strings.Join(strings.Fields(normalizer.output.String()), " ")
}

func (normalizer *textNormalizer) append(character rune) {
	if unicode.Is(unicode.Mn, character) {
		return
	}
	if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
		normalizer.separate()
		return
	}
	if normalizer.previous != 0 && unicode.IsDigit(character) != unicode.IsDigit(normalizer.previous) {
		normalizer.output.WriteByte(' ')
	}
	normalizer.output.WriteRune(character)
	normalizer.previous = character
}

func (normalizer *textNormalizer) separate() {
	normalizer.output.WriteByte(' ')
	normalizer.previous = 0
}

var namedQuantity = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]{1,6})?)\s*(kg|ml|g|l)\b`)

func nameSignature(value string) string {
	value = namedQuantity.ReplaceAllStringFunc(value, normalizeMeasure)
	words := strings.Fields(normalizedText(value))
	sort.Strings(words)
	return strings.Join(words, " ")
}

func normalizeMeasure(measure string) string {
	parts := namedQuantity.FindStringSubmatch(measure)
	size := PackageSize{Quantity: strings.ReplaceAll(parts[1], ",", "."), Unit: strings.ToLower(parts[2])}
	quantity, unit := comparablePackage(&size)
	return quantity.FloatString(6) + " " + unit
}
