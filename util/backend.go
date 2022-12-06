package util

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// Search Backend
var backend *Backend

// setBackend sets the backend value
func SetBackend(be *Backend) {
	backend = be
}

// SetDefaultBackend sets the backend value
// if nothing is fetched based on billing
func SetDefaultBackend() {
	defaultBackend := Zinc
	backend = &defaultBackend
}

// GetBackend returns the backend
func GetBackend() *Backend {
	return backend
}

// IsExternalESRequired will indicate whether or
// not external ES is required.
//
// This will be true if backend is either ES or OS
func IsExternalESRequired() bool {
	if backend == nil {
		return false
	}
	return backend.String() == ElasticSearch.String() || backend.String() == OpenSearch.String()
}

// Backend will be the backend to be used for the knn
// response stage changes.
type Backend int

const (
	ElasticSearch Backend = iota
	OpenSearch
	MongoDB
	Solr
	Fusion
	Zinc
	MarkLogic
)

// String returns the string representation
// of the Backend
func (b Backend) String() string {
	switch b {
	case ElasticSearch:
		return "elasticsearch"
	case OpenSearch:
		return "opensearch"
	case MongoDB:
		return "mongodb"
	case Solr:
		return "solr"
	case Fusion:
		return "fusion"
	case Zinc:
		return "zinc"
	case MarkLogic:
		return "marklogic"
	}
	return ""
}

// UnmarshalJSON is the implementation of Unmarshaler interface to unmarshal the Backend
func (b *Backend) UnmarshalJSON(bytes []byte) error {
	var backend string
	err := json.Unmarshal(bytes, &backend)
	if err != nil {
		return err
	}

	switch backend {
	case OpenSearch.String():
		*b = OpenSearch
	case ElasticSearch.String():
		*b = ElasticSearch
	case MongoDB.String():
		*b = MongoDB
	case Solr.String():
		*b = Solr
	case Fusion.String():
		*b = Fusion
	case Zinc.String():
		*b = Zinc
	case MarkLogic.String():
		*b = MarkLogic
	default:
		return fmt.Errorf("invalid backend passed: %s", backend)
	}

	return nil
}

// MarshalJSON is the implementation of the Marshaler interface to marshal the Backend
func (b Backend) MarshalJSON() ([]byte, error) {
	backend := b.String()

	if backend == "" {
		return nil, fmt.Errorf("invalid backend passed: %s", backend)
	}

	return json.Marshal(backend)
}

func (b Backend) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []interface{}{
			ElasticSearch.String(),
			OpenSearch.String(),
			MongoDB.String(),
			Solr.String(),
			Fusion.String(),
			Zinc.String(),
			MarkLogic.String(),
		},
		Title:       "Backend",
		Description: "Backend that ReactiveSearch will use",
	}
}
