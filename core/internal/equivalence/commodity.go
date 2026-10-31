package equivalence

import (
	"strings"

	"inflacao-real/core/internal/catalog"
)

const RuleVersion = "commodities_v1"

type commodityRule struct {
	name, department string
	aliases          []string
}

var commodityRules = []commodityRule{
	{"banana-nanica", "produce", []string{"banana nanica", "banana caturra", "banana caturra nanica"}},
	{"banana-maca", "produce", []string{"banana maca"}},
	{"banana-prata", "produce", []string{"banana prata"}},
	{"banana-terra", "produce", []string{"banana da terra", "banana terra"}},
	{"batata-inglesa", "produce", []string{"batata inglesa"}},
	{"batata", "produce", []string{"batata"}},
	{"tomate", "produce", []string{"tomate"}},
	{"tomate-italiano", "produce", []string{"tomate italiano"}},
	{"tomate-cereja", "produce", []string{"tomate cereja"}},
	{"cebola", "produce", []string{"cebola"}},
	{"cebola-roxa", "produce", []string{"cebola roxa"}},
	{"cenoura", "produce", []string{"cenoura"}},
	{"patinho", "meat", []string{"patinho", "patinho bovino"}},
	{"coxao-mole", "meat", []string{"coxao mole", "cha de dentro"}},
	{"coxao-duro", "meat", []string{"coxao duro", "cha de fora"}},
}

// CommodityKey consumes every word. Unrecognized wording never establishes identity.
func CommodityKey(product Product) string {
	if !product.Loose || product.Package != nil || len(product.Bases) != 1 || product.Bases[0] != "kg" {
		return ""
	}
	key := commodityName(product.Name, product.Brand)
	if key == "" {
		return ""
	}
	if !consistentListings(product.Listings, key) {
		return ""
	}
	return key
}

func commodityName(name string, brand *string) string {
	normalized := catalog.NormalizeText(name)
	key := ""
	for _, rule := range commodityRules {
		key = firstKey(key, rule.match(normalized, brand))
	}
	return key
}

func (rule commodityRule) match(name string, brand *string) string {
	if !departmentBrand(brand, rule.department) {
		return ""
	}
	key := ""
	for _, alias := range rule.aliases {
		key = firstKey(key, rule.matchAlias(name, alias))
	}
	return key
}

func departmentBrand(brand *string, department string) bool {
	if brand == nil || catalog.NormalizeText(*brand) == "" {
		return true
	}
	allowed := map[string]string{"acougue": "meat", "hortifruti": "produce", "hortifrutigranjeiros": "produce"}
	return allowed[catalog.NormalizeText(*brand)] == department
}

func (rule commodityRule) matchAlias(name, alias string) string {
	remaining, found := removePhrase(name, alias)
	if !found {
		return ""
	}
	attributes, valid := commodityAttributes(remaining, rule.department)
	if !valid {
		return ""
	}
	return RuleVersion + ":" + rule.name + ":" + attributes + ":kg"
}

func removePhrase(name, phrase string) (string, bool) {
	padded := " " + name + " "
	target := " " + phrase + " "
	if !strings.Contains(padded, target) {
		return "", false
	}
	return strings.TrimSpace(strings.Replace(padded, target, " ", 1)), true
}

type commodityAttributesParser struct {
	preparation string
	claims      []string
	valid       bool
}

func commodityAttributes(value, department string) (string, bool) {
	parser := commodityAttributesParser{preparation: "unspecified", valid: true}
	for _, word := range strings.Fields(value) {
		parser.consume(word, department)
	}
	signature := catalog.NameSignature(strings.Join(parser.claims, " "))
	if strings.Contains(signature, "fresh") && strings.Contains(signature, "frozen") {
		return "", false
	}
	return parser.preparation + ":" + signature, parser.valid
}
func (parser *commodityAttributesParser) consume(word, department string) {
	next, claim, accepted := commodityWord(word, department)
	if !accepted || (next != "" && parser.preparation != "unspecified" && next != parser.preparation) {
		parser.valid = false
		return
	}
	if next != "" {
		parser.preparation = next
	}
	if claim != "" {
		parser.claims = append(parser.claims, claim)
	}
}
func consistentListings(listings []Listing, key string) bool {
	consistent := true
	for _, listing := range listings {
		consistent = consistent && commodityName(listing.Name, listing.Brand) == key
	}
	return consistent
}
func firstKey(current, next string) string {
	if current != "" {
		return current
	}
	return next
}

func commodityWord(word, department string) (string, string, bool) {
	if department == "meat" && (word == "carne" || word == "bovina" || word == "bovino" || word == "de") {
		return "", "", true
	}
	if word == "kg" || word == "quilo" {
		return "", "", true
	}
	if word == "organico" || word == "organica" {
		return "", "organic", true
	}
	if word == "congelado" || word == "congelada" {
		return "", "frozen", true
	}
	if word == "fresco" || word == "fresca" {
		return "", "fresh", true
	}
	return meatPreparation(word, department)
}

func meatPreparation(word, department string) (string, string, bool) {
	preparations := map[string]string{"inteiro": "whole", "inteira": "whole", "bife": "steak", "bifes": "steak", "tiras": "strips", "cubos": "cubes", "moido": "ground", "moida": "ground"}
	if department == "meat" && preparations[word] != "" {
		return preparations[word], "", true
	}
	return "", "", false
}
