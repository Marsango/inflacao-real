package catalog

// NormalizeText and NameSignature expose the ingestion normalization for review.
func NormalizeText(value string) string { return normalizedText(value) }
func NameSignature(value string) string { return nameSignature(value) }
func EquivalentPackages(first, second *PackageSize) bool {
	if first == nil || second == nil {
		return false
	}
	return first.equivalent(second)
}
