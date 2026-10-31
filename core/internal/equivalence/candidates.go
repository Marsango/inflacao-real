package equivalence

import (
	"math"
	"sort"
	"strings"

	"inflacao-real/core/internal/catalog"
)

func Suggest(first, second Product) (Candidate, bool) {
	candidate := Candidate{First: first.ID, Second: second.ID, Differences: differingWords(first.Name, second.Name)}
	if first.Representative == second.Representative || !brandedComparable(first, second) {
		return candidate, false
	}
	if conflictingVariants(first.Name, second.Name) {
		return candidate, false
	}
	candidate.Score = tokenSimilarity(first.Name, second.Name)
	candidate.Kind, candidate.Reason = "fuzzy", "Similaridade de palavras; confirme as fontes e as variantes."
	if catalog.NameSignature(first.Name) == catalog.NameSignature(second.Name) {
		candidate.Kind, candidate.Score, candidate.Reason = "exact", 1, "Nome, marca e embalagem normalizados equivalentes; códigos podem divergir."
	}
	candidate.VerificationRequired = len(candidate.Differences) > 0
	return candidate, candidate.Score >= 0.80
}

func brandedComparable(first, second Product) bool {
	return first.Brand != nil && second.Brand != nil && catalog.NormalizeText(*first.Brand) != "" &&
		catalog.NormalizeText(*first.Brand) == catalog.NormalizeText(*second.Brand) &&
		catalog.EquivalentPackages(first.Package, second.Package) && sameBases(first.Bases, second.Bases)
}

func sameBases(first, second []string) bool {
	return strings.Join(first, ",") == strings.Join(second, ",")
}

var variantFamilies = []string{
	"suave forte extra extraforte tradicional", "chocolate baunilha morango caramelo coco limao laranja uva maracuja",
	"descafeinado cafeinado", "integral desnatado semidesnatado", "vacuo almofada capsula capsulas sache vidro lata pet",
}

func conflictingVariants(first, second string) bool {
	if numericWords(first) != numericWords(second) {
		return true
	}
	first, second = catalog.NormalizeText(first), catalog.NormalizeText(second)
	if dietaryClaims(first) != dietaryClaims(second) {
		return true
	}
	conflicting := false
	for _, family := range variantFamilies {
		conflicting = conflicting || conflictingFamily(first, second, family)
	}
	return conflicting
}

func dietaryClaims(name string) string {
	claims := []string{}
	for _, claim := range []string{"zero", "diet", "light", "sem lactose", "sem gluten", "sem acucar", "organico", "organica", "vegano", "vegana"} {
		claims = includeWord(claims, name, claim)
	}
	return strings.Join(claims, ",")
}

func conflictingFamily(first, second, family string) bool {
	left, right := familyWords(first, family), familyWords(second, family)
	return left != "" && right != "" && left != right
}

func familyWords(name, family string) string {
	words := []string{}
	for _, word := range strings.Fields(family) {
		words = includeWord(words, name, word)
	}
	return strings.Join(words, " ")
}

func numericWords(name string) string {
	numbers := []string{}
	for _, word := range strings.Fields(catalog.NameSignature(name)) {
		numbers = includeNumeric(numbers, word)
	}
	return strings.Join(numbers, " ")
}

func wordSet(name string) []string {
	words := strings.Fields(catalog.NameSignature(name))
	return compact(words)
}

func compact(words []string) []string {
	result := []string{}
	for _, word := range words {
		result = appendUnique(result, word)
	}
	return result
}

func differingWords(first, second string) []string {
	left, right := wordSet(first), wordSet(second)
	result := []string{}
	for _, word := range append(left, right...) {
		result = appendDifference(result, left, right, word)
	}
	sort.Strings(result)
	return result
}

func contains(words []string, word string) bool {
	position := sort.SearchStrings(words, word)
	return position < len(words) && words[position] == word
}

func tokenSimilarity(first, second string) float64 {
	left, right := wordSet(first), wordSet(second)
	matched := 0
	for _, word := range left {
		right, matched = matchToken(word, right, matched)
	}
	denominator := len(left) + len(wordSet(second))
	if denominator == 0 {
		return 0
	}
	return math.Round(20000*float64(matched)/float64(denominator)) / 10000
}

func closestToken(word string, words []string) int {
	selected := -1
	for position, other := range words {
		selected = selectToken(selected, position, word, other)
	}
	return selected
}

func editDistance(first, second string) int {
	previous := make([]int, len(second)+1)
	for position := range previous {
		previous[position] = position
	}
	for position := range first {
		previous = distanceRow(first[position], position+1, second, previous)
	}
	return previous[len(second)]
}

func distanceRow(character byte, index int, second string, previous []int) []int {
	current := make([]int, len(second)+1)
	current[0] = index
	for position := range second {
		cost := substitutionCost(character, second[position])
		current[position+1] = min(current[position]+1, previous[position+1]+1, previous[position]+cost)
	}
	return current
}

func includeWord(words []string, name, word string) []string {
	if strings.Contains(" "+name+" ", " "+word+" ") {
		return append(words, word)
	}
	return words
}
func includeNumeric(words []string, word string) []string {
	if strings.ContainsAny(word, "0123456789") {
		return append(words, word)
	}
	return words
}
func appendUnique(words []string, word string) []string {
	if len(words) == 0 || words[len(words)-1] != word {
		return append(words, word)
	}
	return words
}
func appendDifference(words, left, right []string, word string) []string {
	if contains(left, word) != contains(right, word) {
		return append(words, word)
	}
	return words
}
func matchToken(word string, right []string, matched int) ([]string, int) {
	position := closestToken(word, right)
	if position < 0 {
		return right, matched
	}
	return append(right[:position], right[position+1:]...), matched + 1
}
func selectToken(selected, position int, word, other string) int {
	if selected >= 0 {
		return selected
	}
	if word == other || (len(word) >= 5 && len(other) >= 5 && editDistance(word, other) <= 1) {
		return position
	}
	return selected
}
func substitutionCost(first, second byte) int {
	if first == second {
		return 0
	}
	return 1
}
