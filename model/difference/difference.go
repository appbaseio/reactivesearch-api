package difference

import "github.com/sergi/go-diff/diffmatchpatch"

type Difference struct {
	URI     []diffmatchpatch.Diff `json:"uri"`
	Headers []diffmatchpatch.Diff `json:"headers"`
	Body    []diffmatchpatch.Diff `json:"body"`
	Method  []diffmatchpatch.Diff `json:"method"`
	Stage   string                `json:"stage"`
	Took    *float64              `json:"took,omitempty"`
}
