package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/tracktime"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
)

const logTag = "[logger]"

// Log logs and records time taken by each requests. As a side effect,
// it trims the trailing slashes from the matched route.
func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// avoid logging requests for the following endpoints
		if strings.Contains(req.RequestURI, "/arc/health") || strings.Contains(req.RequestURI, "/arc/_health") {
			next.ServeHTTP(w, req)
			return
		}

		// Avoid logging for the SSE endpoints as well

		// NOTE: Trying to fetch the route template from mux
		// doesn't work here since route is a pointer and fetching
		// it at this point returns a nil value.
		//
		// Thus, we are using regex match here to check if it is the SSE
		// endpoint.
		if util.IsSSEEndpoint(req) {
			next.ServeHTTP(w, req)
			log.Println(fmt.Sprintf("%s: finished %s", logTag, fmt.Sprintf("%s %s", req.Method, req.URL.Path)))
			return
		}

		ctx := tracktime.NewTimeTrackerContext(req.Context())
		req = req.WithContext(ctx)

		userIdUsed := iplookup.FromRequest(req)
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(req.Body)
		if err != nil {
			log.Warnln(logTag, ":", err)
		}

		bodyInBytes := buf.Bytes()

		req.Body = io.NopCloser(buf)

		req.URL.Path = trimTrailingSlashes(req.URL.Path)

		// Create recorder to capture the response and modify it with
		// the final time taken.
		responseRecorder := httptest.NewRecorder()
		next.ServeHTTP(responseRecorder, req)

		responseBodyInBytes := responseRecorder.Body.Bytes()

		// Copy the response to writer
		for k, v := range responseRecorder.Header() {
			w.Header()[k] = v
		}

		// Fetch the time from the time tracker
		startAsPtr, fetchErr := tracktime.FromTimeTrackerContext(req.Context())
		if fetchErr != nil {
			log.Warnln(logTag, ": error while fetching time from ctx to calculate time taken: ", fetchErr.Error())
			return
		}
		start := *startAsPtr

		timeTakenForRequest := time.Since(start).Milliseconds()

		// Add the usual log
		log.Println(fmt.Sprintf("%s: finished %s, took %dms",
			logTag, fmt.Sprintf("%s %s", req.Method, req.URL.Path), timeTakenForRequest))

		// Inject the time taken into the response if the request is of type _reactivesearch
		//
		// Make sure that the route is _reactivesearch and it has a valid 200 OK response code
		isRSRoute := strings.HasSuffix(req.URL.Path, "/_reactivesearch") || strings.HasSuffix(req.URL.Path, "/_reactivesearch.v3")
		if !isRSRoute || responseRecorder.Code != http.StatusOK {
			// Write the body and return
			w.WriteHeader(responseRecorder.Code)
			w.Write(responseBodyInBytes)
			return
		}

		// Inject the time taken now.
		w.Header().Set("X-Took", fmt.Sprintf("%d", timeTakenForRequest))

		timeTakenInBytes, marshalErr := json.Marshal(timeTakenForRequest)
		if marshalErr != nil {
			log.Warnln(logTag, ": error while marshalling time taken into json: ", marshalErr.Error())
			return
		}

		// Extract the `settings.took` value and set it into a different key
		tookValue, tookReadErr := jsonparser.GetFloat(responseBodyInBytes, "settings", "took")
		if tookReadErr != nil {
			tookValue = 0
		}

		updatedBodyInBytes, setErr := jsonparser.Set(responseBodyInBytes, timeTakenInBytes, "settings", "took")
		if setErr != nil {
			log.Warnln(logTag, ": error while injecting time taken into response: ", setErr.Error())
			return
		}

		// Determine what the name of the older took should be. It can be `searchTook` or
		// `pipelineTook`.
		//
		// We can check this by checking whether settings.pipelineId is present or not.
		olderTookKeyName := "searchTook"
		pipelineId, readErr := jsonparser.GetString(updatedBodyInBytes, "settings", "pipelineId")
		if readErr == nil && strings.TrimSpace(pipelineId) != "" {
			olderTookKeyName = "pipelineTook"
		}

		updatedBodyInBytes, tookSetErr := jsonparser.Set(updatedBodyInBytes, []byte(fmt.Sprintf("%.0f", tookValue)), "settings", olderTookKeyName)
		if tookSetErr != nil {
			log.Warnln(logTag, ": error while setting took in response body: ", tookSetErr.Error())
		}

		// Inject the `userId` used in the codebase for this particular request.
		//
		// Throughout the codebase, we use the `settings.userId` as the userId wherever
		// required and use the IP address of the request as a fallback.
		// Read the userId from the request body if it is passed.

		// Try to read the userId from settings
		readUserId, userIdReadErr := jsonparser.GetString(bodyInBytes, "settings", "userId")
		if userIdReadErr == nil {
			userIdUsed = readUserId
		}

		var userIdSetErr error
		updatedBodyInBytes, userIdSetErr = jsonparser.Set(updatedBodyInBytes, []byte(fmt.Sprintf("%q", userIdUsed)), "settings", "userId")
		if userIdSetErr != nil {
			log.Warnln(logTag, ": ", "error while setting userId in response body: ", userIdSetErr.Error())
		}

		w.WriteHeader(responseRecorder.Code)
		w.Write(updatedBodyInBytes)
	})
}

func trimTrailingSlashes(path string) string {
	for path != "/" && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}
