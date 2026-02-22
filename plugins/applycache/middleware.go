package applycache

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/model/tracktime"
	"github.com/appbaseio/reactivesearch-api/plugins/analytics"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/cache"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/openai"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
}

// A list of plans for which this feature will be available
var validPlans = []util.Plan{
	util.ArcEnterprise,
	util.HostedArcEnterprise,
	util.ProductionFirst2019,
	util.ProductionSecond2019,
	util.ProductionThird2019,
	util.ProductionFourth2019,
	// 2021 plans
	util.ProductionFirst2021,
	util.ProductionSecond2021,
	util.ProductionThird2021,
	util.HostedArcEnterprise2021,
	util.ProductionFirst2023,
}

// Middleware with custom filters (plan based) restriction
func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, append(list(), validatePlan)...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classify.Op(),
		classify.Indices(),
		logs.Recorder(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.Analytics

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func validatePlan(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		restrictedRoutes := []string{
			"filter-labels",
			"filter-values",
			"insights",
			"insight-status",
		}
		var isRestrictedRoute bool
		for _, route := range restrictedRoutes {
			if strings.Contains(req.RequestURI, route) {
				isRestrictedRoute = true
				break
			}
		}

		// Throw error if custom filters are present in the URL
		if (isRestrictedRoute || len(analytics.GetCustomFilters(req.URL.Query())) != 0) && !util.ValidatePlans(validPlans, util.GetFeatureCustomEvents()) {
			msg := "Custom filters feature is not available for the free plan users, please upgrade to a paid plan or remove custom filter."
			if util.GetTier() != nil {
				msg = "Custom filters feature is not available for the " + util.GetTier().String() + " plan users, please upgrade to a higher plan or remove custom filter."
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusPaymentRequired)
		} else {
			h(w, req)
		}
	}
}

// Plugin to track cache
func trackCachePlugin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		body, err := querytranslate.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}
		if cache.ShouldApplyCache(req, *body) {
			// Track plugin
			ctx := trackplugin.TrackPlugin(ctx, "ca")
			r := req.WithContext(ctx)
			h(w, r)
		} else {
			h(w, req)
		}
	}
}

// It should be the last request middleware
func applyCachedResponse(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := querytranslate.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		isCacheHit := false
		cacheInstance := Instance()

		if cache.ShouldApplyCache(req, *body) {
			startTime, err := tracktime.FromTimeTrackerContext(req.Context())
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request time from context", http.StatusInternalServerError)
				return
			}

			indices, err := index.FromContext(req.Context())
			if err != nil {
				msg := "error getting the index names from context"
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}

			cachedResponse, err := ApplyCache(req.RequestURI, body, nil, startTime, iplookup.FromRequest(req), indices)
			if err != nil {
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}

			// This is where the cache is being written as response
			// We will consider the cache a hit only when it reaches this phase.
			if cachedResponse != nil {
				// Capture the cache as a hit
				cacheInstance.rollOver.Add(true, cachedResponse.PerformanceSave)
				isCacheHit = true

				for k, v := range cachedResponse.Headers {
					w.Header().Set(k, v)
				}
				util.WriteBackRaw(w, cachedResponse.Body, http.StatusOK)
				return
			}
		}

		// If cache is not hit, count it as a miss
		if !isCacheHit {
			cacheInstance.rollOver.Add(false, 0)
		}

		h(w, req)
	}
}

type CachedResponse struct {
	Body            []byte
	Headers         map[string]string
	PerformanceSave int64
}

func ApplyCache(
	urlPath string,
	rsAPIBody *querytranslate.RSQuery,
	requestBody []byte,
	startTime *time.Time,
	userId string,
	indices []string,
) (*CachedResponse, error) {
	// Initialize 'out' with the request body
	out := requestBody

	// If 'rsAPIBody' is provided, sanitize and marshal it
	if rsAPIBody != nil {
		sanitizedRequest := cache.SanitizeRequest(*rsAPIBody)
		output, err := json.Marshal(sanitizedRequest)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, err
		}
		out = output
	}

	var performanceSave int

	// Generate the cache key and attempt to read from cache
	cacheKey, valueAsByte := cache.ReadResponseFromCache(urlPath, out)
	if valueAsByte != nil {
		headers := map[string]string{
			cache.CachedRequestHeader: "true",
		}
		var responseToWrite []byte
		modifiedValue := valueAsByte // Start with the original cached response

		// Check if OpenAI is enabled for the given indices
		if IsOpenAIEnabled(indices) {
			sessionMap := openai.SessionInstance()

			// Iterate over the cached response to update session IDs
			sessionIdUpdateErr := jsonparser.ObjectEach(valueAsByte, func(key, value []byte, dataType jsonparser.ValueType, offset int) error {
				// Skip reserved keys
				if !util.ContainsBytes(querytranslate.RESERVED_KEYS_IN_RESPONSE_BYTES, key) {
					// Attempt to retrieve the session ID from the cached response
					sessionDetails, valueType, _, sessionIdParseErr := jsonparser.Get(valueAsByte, string(key), querytranslate.SessionIDKeyToInject)
					if sessionIdParseErr != nil {
						if sessionIdParseErr == jsonparser.KeyPathNotFoundError {
							log.Warnln(logTag, ": error finding sessionId in object: ", sessionIdParseErr.Error())
							return nil
						}
						return fmt.Errorf("error while parsing sessionId from cached response: %w", sessionIdParseErr)
					}

					var newSessionId string

					// Check if the session details are valid
					if valueType != jsonparser.Object {
						// Execute the AI Answer query since the previous call failed
						updatedRSResponse, AIAnswerErr := querytranslate.ExecuteAIAnswerInQuery(rsAPIBody, responseToWrite, indices, sessionMap, userId)
						if AIAnswerErr != nil {
							errMsg := fmt.Sprintf(": error while executing AI Answer query: %v", AIAnswerErr)
							log.Warnln(logTag, errMsg)
							return fmt.Errorf(errMsg)
						}
						responseToWrite = updatedRSResponse

						// Extract the new session ID
						extractedSessionId, readErr := jsonparser.GetString(updatedRSResponse, string(key), querytranslate.SessionIDKeyToInject)
						if readErr != nil {
							errMsg := fmt.Sprintf("sessionId not injected, cannot continue with cache hit: %v", readErr)
							log.Warnln(logTag, ": ", errMsg)
							return fmt.Errorf(errMsg)
						}

						newSessionId = extractedSessionId

						// Update the cache with the new session ID and response asynchronously
						go func(key string, value []byte) {
							responseDetails := sessionMap.GetResponse(newSessionId)
							if responseDetails == nil {
								log.Warnln(logTag, ": not updating cache since response is nil!")
								return
							}

							// Wait for the response to be ready
							for !responseDetails.GetIsReady() {
								time.Sleep(5 * time.Second)
							}

							if responseDetails.GetIsFailed() {
								log.Debugln(logTag, ": not updating cache since AI response failed!")
								return
							}

							// Update the cache with the AI answer
							defaultMaxDuration := int64(0)
							requestSize := int64(len(cacheKey))
							responseSize := int64(len(value))
							itemCost := requestSize + responseSize

							cache.UpdateValueWithAIAnswer(cacheKey, key, responseDetails.Response(), responseDetails, value, defaultMaxDuration, itemCost)
						}(string(key), value)
					} else {
						// Generate a new session ID based on the existing session details
						var newSessionGenerateErr error
						newSessionId, newSessionGenerateErr = sessionMap.SessionFromBytes(sessionDetails, userId)
						if newSessionGenerateErr != nil {
							errMsg := fmt.Sprintf("error while generating new session Id from details: %v", newSessionGenerateErr)
							log.Warnln(logTag, ": ", errMsg)
							return fmt.Errorf(errMsg)
						}
					}

					// Inject the new session ID into the modified response
					bodyWithNewSessionId, injectErr := jsonparser.Set(modifiedValue, []byte(fmt.Sprintf(`"%s"`, newSessionId)), string(key), querytranslate.SessionIDKeyToInject)
					if injectErr != nil {
						errMsg := fmt.Sprintf("error while injecting new sessionId: %v", injectErr)
						log.Warnln(logTag, ": ", errMsg)
						return fmt.Errorf(errMsg)
					}

					modifiedValue = bodyWithNewSessionId
					return nil
				}
				return nil
			})

			if sessionIdUpdateErr != nil {
				errMsg := fmt.Sprintf("error while updating sessionId in cached response: %v", sessionIdUpdateErr)
				log.Warnln(logTag, ": ", errMsg)
				return nil, fmt.Errorf(errMsg)
			}
		}

		// Proceed to modify the 'took' and 'cached' values in the response
		if rsAPIBody != nil {
			took := time.Since(*startTime).Milliseconds()

			// Retrieve the original 'took' value
			originalTook, _, _, err := jsonparser.Get(modifiedValue, "settings", "took")
			if err == nil {
				originalTookAsInt, tookToIntErr := strconv.Atoi(string(originalTook))
				if tookToIntErr != nil {
					log.Warnln(logTag, ": error while converting original took to int: ", tookToIntErr)
				} else {
					performanceSave = originalTookAsInt - int(took)
				}
			}

			// Update the 'took' value in the response
			responseBody, err := jsonparser.Set(modifiedValue, []byte(fmt.Sprintf("%d", took)), "settings", "took")
			if err != nil {
				responseBody2, err2 := jsonparser.Set(modifiedValue, []byte(fmt.Sprintf(`{ "took": %d }`, took)), "settings")
				if err2 != nil {
					log.Warnln(logTag, "unable to set settings.took key: ", err2)
				} else {
					responseToWrite = responseBody2
				}
			} else {
				responseToWrite = responseBody
			}

			// Add the 'cached' flag to the settings
			responseToWrite, err = jsonparser.Set(responseToWrite, []byte("true"), "settings", "cached")
			if err != nil {
				responseToWrite, err = jsonparser.Set(responseToWrite, []byte(`{ "cached": true }`), "settings")
				if err != nil {
					log.Warnln(logTag, "unable to set settings.cached key: ", err)
				}
			}
		} else {
			responseToWrite = modifiedValue
		}

		// Return the cached response
		return &CachedResponse{
			Body:            responseToWrite,
			Headers:         headers,
			PerformanceSave: int64(performanceSave),
		}, nil
	}
	// Return nil if no cached response is available
	return nil, nil
}
