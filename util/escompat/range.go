package escompat

import es7 "github.com/olivere/elastic/v7"

// RangeQuery is a minimal replacement that always serializes to gte/lte/gt/lt.
type RangeQuery struct {
	field    string
	gte, gt  interface{}
	lte, lt  interface{}
	format   string
	timeZone string
}

func NewRangeQuery(field string) *RangeQuery        { return &RangeQuery{field: field} }
func (q *RangeQuery) Gte(v interface{}) *RangeQuery { q.gte = v; return q }
func (q *RangeQuery) Lte(v interface{}) *RangeQuery { q.lte = v; return q }
func (q *RangeQuery) Gt(v interface{}) *RangeQuery  { q.gt = v; return q }
func (q *RangeQuery) Lt(v interface{}) *RangeQuery  { q.lt = v; return q }
func (q *RangeQuery) Format(v string) *RangeQuery   { q.format = v; return q }
func (q *RangeQuery) TimeZone(v string) *RangeQuery { q.timeZone = v; return q }

// Ensure interface compliance with elastic.Query
var _ es7.Query = (*RangeQuery)(nil)

func (q *RangeQuery) Source() (interface{}, error) {
	params := map[string]interface{}{}
	if q.gte != nil {
		params["gte"] = q.gte
	}
	if q.gt != nil {
		params["gt"] = q.gt
	}
	if q.lte != nil {
		params["lte"] = q.lte
	}
	if q.lt != nil {
		params["lt"] = q.lt
	}
	if q.format != "" {
		params["format"] = q.format
	}
	if q.timeZone != "" {
		params["time_zone"] = q.timeZone
	}

	return map[string]interface{}{
		"range": map[string]interface{}{
			q.field: params,
		},
	}, nil
}
