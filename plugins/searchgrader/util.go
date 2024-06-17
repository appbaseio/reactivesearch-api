package searchgrader

import (
	"os"
)

// ESDoc represents the ES document structure for search grader
type ESDoc struct {
	Index string  `json:"index,omitempty"`
	Query *string `json:"query,omitempty"`
	DocID *string `json:"doc_id,omitempty"`
	Grade *int    `json:"grade,omitempty"`
}

// ESRecord represents the document to be updated to ES
type ESRecord struct {
	Index string  `json:"index,omitempty"`
	Query *string `json:"query,omitempty"`
	DocID *string `json:"doc_id,omitempty"`
	Grade *int    `json:"grade,omitempty"`
}

// GradeRequest represents the request body for grade document API
type GradeRequest struct {
	Query *string `json:"query,omitempty"`
	Grade *int    `json:"grade,omitempty"`
}

// UpdateGradeOptions represents the options for updating grade
type UpdateGradeOptions struct {
	DocID        string
	Script       string
	Record       map[string]interface{}
	ScriptParams map[string]interface{}
}

// GradeMetricsRequest represents the request body struct for grade metrics endpoint
type GradeMetricsRequest struct {
	Indices []string `json:"indices"`
	Page    *int     `json:"page,omitempty"`
}

// GradeMetricsResponse represents the response body struct for grade metrics endpoint
type GradeMetricsResponse struct {
	Total   int                       `json:"total"`
	Metrics map[string]map[string]int `json:"metrics"`
}

const idSeparator = "__"

func generateDocID(docID string, query string) string {
	return docID + idSeparator + query
}

func getInterfaceArray(t []string) []interface{} {
	s := make([]interface{}, len(t))
	for i, v := range t {
		s[i] = v
	}
	return s
}

func getMasterCredentials() string {
	username, password := os.Getenv("USERNAME"), os.Getenv("PASSWORD")
	if username == "" {
		username, password = "foo", "bar"
	}
	return username + ":" + password
}
