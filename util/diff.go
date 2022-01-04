package util

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/prometheus/common/log"
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
