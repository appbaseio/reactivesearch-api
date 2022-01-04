package util

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/model/difference"
	"github.com/prometheus/common/log"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Deep clone the request body by also reading the body and keeping
// the body back in the original one.
func DeepCloneRequest(req *http.Request) (*http.Request, error) {
	copiedRequest := req.Clone(req.Context())

	copiedBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorln(" error while reading body from request, ", err)
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(copiedBody))
	copiedRequest.Body = ioutil.NopCloser(bytes.NewReader(copiedBody))

	return copiedRequest, nil
}

// Calculate the diff between the passed bodies.
// We will find the difference between
// - body
// - headers
// - URI
// - method
func CalculateDiff(originalReq *http.Request, modifiedReq *http.Request) *difference.Difference {
	// Convert the requests to strings and then find the diff
	bodyDiffStr := CalculateBodyDiff(originalReq, modifiedReq)
	headerDiffStr := CalculateHeaderDiff(originalReq, modifiedReq)
	uriDiffStr := CalculateUriDiff(originalReq, modifiedReq)
	methodDiffStr := CalculateMethodDiff(originalReq, modifiedReq)

	return &difference.Difference{
		URI:     uriDiffStr,
		Headers: headerDiffStr,
		Method:  methodDiffStr,
		Body:    bodyDiffStr,
	}
}

// Calculate the diff in the body
func CalculateBodyDiff(originalReq *http.Request, modifiedReq *http.Request) string {
	bodyReadBuffer := new(bytes.Buffer)
	bodyReadBuffer.ReadFrom(originalReq.Body)
	originalBodyStr := bodyReadBuffer.String()

	bodyReadBuffer.ReadFrom(modifiedReq.Body)
	modifiedBodyStr := bodyReadBuffer.String()

	dmp := diffmatchpatch.New()
	bodyDiffs := dmp.DiffMain(originalBodyStr, modifiedBodyStr, false)
	bodyDiffStr := dmp.DiffPrettyText(bodyDiffs)

	return bodyDiffStr
}

// Calculate the difference in the URI
func CalculateUriDiff(originalReq *http.Request, modifiedReq *http.Request) string {
	dmp := diffmatchpatch.New()
	URIDiffs := dmp.DiffMain(originalReq.URL.Path, modifiedReq.URL.Path, false)
	log.Debug(": URI diff calculated, ", dmp.DiffPrettyText(URIDiffs))
	return dmp.DiffPrettyText(URIDiffs)
}

// Calculate method difference
func CalculateMethodDiff(originalReq *http.Request, modifiedReq *http.Request) string {
	dmp := diffmatchpatch.New()
	methodDiff := dmp.DiffMain(originalReq.Method, modifiedReq.Method, false)
	return dmp.DiffPrettyText(methodDiff)
}

func CalculateHeaderDiff(originalReq *http.Request, modifiedReq *http.Request) string {
	originalHeaders, err := json.Marshal(originalReq.Header)
	if err != nil {
		log.Warnln(" could not marshal original request headers, ", err)
	}

	// Marshal the modified request headers
	modifiedHeaders, err := json.Marshal(modifiedReq.Header)
	if err != nil {
		log.Warnln(" could not marshal modified request headers, ", err)
	}

	dmp := diffmatchpatch.New()
	headerDiff := dmp.DiffMain(string(originalHeaders), string(modifiedHeaders), false)

	log.Debug(": Header diff calculated, ", dmp.DiffPrettyText(headerDiff))
	return dmp.DiffPrettyText(headerDiff)
}
