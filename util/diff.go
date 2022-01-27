package util

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/appbaseio/reactivesearch-api/model/difference"
	"github.com/gdexlab/go-render/render"
	"github.com/sergi/go-diff/diffmatchpatch"
	log "github.com/sirupsen/logrus"
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

// Deep clone the response body by also reading the body and keeping the
// body back in the original response.
func DeepCloneResponse(res *httptest.ResponseRecorder) (*httptest.ResponseRecorder, error) {
	// Since there is no clone method, we need to create a new recorder
	// and copy all the fields there.
	copiedResponse := new(httptest.ResponseRecorder)

	buffer := new(bytes.Buffer)
	_, err := buffer.ReadFrom(res.Body)
	if err != nil {
		log.Errorln(" error while reading body from response, ", err)
	}
	copiedResponse.Body = bytes.NewBufferString(buffer.String())
	res.Body = bytes.NewBufferString(buffer.String())

	// Copy the headers as well
	copiedResponse.Result().Header = res.Header().Clone()

	return copiedResponse, nil
}

// Calculate the diff between the passed bodies.
// We will find the difference between
// - body
// - headers
// - URI
// - method
func CalculateRequestDiff(originalReq *http.Request, modifiedReq *http.Request) *difference.Difference {
	// Convert the requests to strings and then find the diff
	bodyDiffStr := CalculateBodyDiff(originalReq.Body, modifiedReq.Body)
	headerDiffStr := CalculateHeaderDiff(originalReq.Header, modifiedReq.Header)
	uriDiffStr := CalculateUriDiff(originalReq, modifiedReq)
	methodDiffStr := CalculateMethodDiff(originalReq, modifiedReq)

	return &difference.Difference{
		URI:     uriDiffStr,
		Headers: headerDiffStr,
		Method:  methodDiffStr,
		Body:    bodyDiffStr,
	}
}

// Calculate the diff between the passed bodies.
// We will find the difference between
// - body.
// - headers
func CalculateResponseDiff(originalRes *httptest.ResponseRecorder, modifiedRes *httptest.ResponseRecorder) *difference.Difference {
	bodyDiffStr := CalculateBodyDiff(originalRes.Result().Body, modifiedRes.Result().Body)
	headerDiffStr := CalculateHeaderDiff(originalRes.Result().Header, modifiedRes.Result().Header)

	return &difference.Difference{
		Body:    bodyDiffStr,
		Headers: headerDiffStr,
	}
}

// Calculate the diff in the body
func CalculateBodyDiff(originalReqBody io.ReadCloser, modifiedReqBody io.ReadCloser) string {
	bodyReadBuffer := new(bytes.Buffer)
	bodyReadBuffer.ReadFrom(originalReqBody)
	originalBodyStr := bodyReadBuffer.String()

	// Reset the buffer else the data will be appended
	// which will create weird side effects
	bodyReadBuffer.Reset()

	bodyReadBuffer.ReadFrom(modifiedReqBody)
	modifiedBodyStr := bodyReadBuffer.String()

	log.Debug("Comparing: ", render.AsCode(originalBodyStr), " with: ", render.AsCode(modifiedBodyStr))

	dmp := diffmatchpatch.New()
	bodyDiffs := dmp.DiffMain(originalBodyStr, modifiedBodyStr, false)
	bodyDiffStr := dmp.DiffToDelta(bodyDiffs)

	// Make an integrity check to make sure the delta
	// is not compromised
	if log.GetLevel() == log.DebugLevel {
		_, err := dmp.DiffFromDelta(originalBodyStr, bodyDiffStr)
		if err != nil {
			log.Warnln("Integrity check failed for body, couldn't build diffs from delta")
			return ""
		}
	}

	return bodyDiffStr
}

// Calculate the difference in the URI
func CalculateUriDiff(originalReq *http.Request, modifiedReq *http.Request) string {
	dmp := diffmatchpatch.New()
	URIDiffs := dmp.DiffMain(originalReq.URL.Path, modifiedReq.URL.Path, false)
	return dmp.DiffToDelta(URIDiffs)
}

// Calculate method difference
func CalculateMethodDiff(originalReq *http.Request, modifiedReq *http.Request) string {
	dmp := diffmatchpatch.New()
	methodDiff := dmp.DiffMain(originalReq.Method, modifiedReq.Method, false)
	return dmp.DiffToDelta(methodDiff)
}

func CalculateHeaderDiff(originalReqHeader http.Header, modifiedReqHeader http.Header) string {
	originalHeaders, err := json.Marshal(originalReqHeader)
	if err != nil {
		log.Warnln(" could not marshal original request headers, ", err)
	}

	// Marshal the modified request headers
	modifiedHeaders, err := json.Marshal(modifiedReqHeader)
	if err != nil {
		log.Warnln(" could not marshal modified request headers, ", err)
	}

	dmp := diffmatchpatch.New()
	headerDiff := dmp.DiffMain(string(originalHeaders), string(modifiedHeaders), false)
	return dmp.DiffToDelta(headerDiff)
}
