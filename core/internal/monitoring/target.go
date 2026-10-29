package monitoring

type Target struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	SearchQueries []string   `json:"search_queries"`
	RequiredTerms [][]string `json:"required_terms"`
	ExcludedTerms []string   `json:"excluded_terms"`
}

type Scope struct {
	SchemaVersion string   `json:"schema_version"`
	Targets       []Target `json:"targets"`
}
