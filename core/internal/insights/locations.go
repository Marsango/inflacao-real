package insights

import (
	"sort"
	"strings"
)

func observedLocations(series []ProductSeries) string {
	locations := map[string]bool{}
	for _, product := range series {
		collectLocations(locations, product.Contexts)
	}
	names := make([]string, 0, len(locations))
	for name := range locations {
		names = append(names, name)
	}
	sort.Strings(names)
	return " Localidades observadas: " + strings.Join(names, "; ") + "."
}

func collectLocations(locations map[string]bool, contexts []ContextSeries) {
	for _, context := range contexts {
		locations[observedLocation(context)] = true
	}
}

func observedLocation(context ContextSeries) string {
	if context.Location == "" {
		return "localidade não informada"
	}
	return context.Location
}
