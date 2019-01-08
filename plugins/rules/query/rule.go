package query

// Query should effectively translate to: { "query: { "regexp": { "condition.pattern": "%s" } } }.
// It represents a regexp query which when executed against the search term/doc identifies the
// rule associated with it.
type Query struct {
	Regexp struct {
		Pattern string `json:"condition.pattern"`
	} `json:"regexp,omitempty"`
}

// Rule represents a Query Rule. A query rule consists of a condition which
// if the search request fulfils triggers the consequence. The query rules are
// basically if-this-then-that construct i.e. if "condition" is true then
// trigger the "consequence". Each rule can be identified by a query. The query
// is run against the incoming search queries and the consequence is triggered if
// the rule applies to the search query.
type Rule struct {
	Query       Query       `json:"query,omitempty"`
	Condition   Condition   `json:"condition"`
	Consequence Consequence `json:"consequence"`
}
