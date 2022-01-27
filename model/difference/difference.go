package difference

type Difference struct {
	URI     string   `json:"uri"`
	Headers string   `json:"headers"`
	Body    string   `json:"body"`
	Method  string   `json:"method"`
	Stage   string   `json:"stage"`
	Took    *float64 `json:"took,omitempty"`
}
