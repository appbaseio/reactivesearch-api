package applycache

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/middleware/validate"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/model/index"
	"github.com/appbaseio-confidential/reactivesearch/model/trackplugin"
	"github.com/appbaseio-confidential/reactivesearch/model/tracktime"
	"github.com/appbaseio-confidential/reactivesearch/plugins/analytics"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
	"github.com/appbaseio-confidential/reactivesearch/plugins/cache"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
	"github.com/appbaseio-confidential/reactivesearch/plugins/openai"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/appbaseio-confidential/reactivesearch/util/iplookup"
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

func ApplyCache(urlPath string, rsAPIBody *querytranslate.RSQuery, requestBody []byte, startTime *time.Time, userId string, indices []string) (*CachedResponse, error) {
	out := string(requestBody)
	if rsAPIBody != nil {
		sanitizedRequest := cache.SanitizeRequest(*rsAPIBody)
		output, err := json.Marshal(sanitizedRequest)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, err
		}
		out = string(output)
	}

	var performanceSave int

	value := cache.ReadResponseFromCache(urlPath, out)
	if value != nil {
		ValueAsByte, ok := value.([]byte)
		if !ok {
			valueAsString, ok := value.(string)
			if !ok {
				log.Errorln(logTag, ":", value.(string))
				return nil, errors.New("error occurred while reading the cached response")
			}
			ValueAsByte = []byte(valueAsString)
		}
		headers := map[string]string{}
		// Set cache header
		headers[cache.CachedRequestHeader] = "true"
		var responseToWrite []byte

		// Before checking if OpenAI session replacement should happen, we will
		// need to make some checks to see if OpenAI is even enabled in the cluster.
		if IsOpenAIEnabled(indices) {
			log.Println("ERROR: openAI is marked as available.. debug more")
			// Iterate the objects and parse the sessionId. If found, replace the sessionId
			// with a new one.
			copiedResponse := make([]byte, len(ValueAsByte))
			copy(copiedResponse, ValueAsByte)
			sessionMap := openai.SessionInstance()
			sessionIdUpdateErr := jsonparser.ObjectEach(copiedResponse, func(key, value []byte, dataType jsonparser.ValueType, offset int) error {
				if !util.Contains(querytranslate.RESERVED_KEYS_IN_RESPONSE, string(key)) {
					// Check if sessionId is present
					sessionDetails, valueType, _, sessionIdParseErr := jsonparser.Get(ValueAsByte, string(key), querytranslate.SessionIDKeyToInject)
					if sessionIdParseErr != nil {
						// Ignore if not found error is reported
						if sessionIdParseErr == jsonparser.KeyPathNotFoundError {
							log.Warnln(logTag, ": error finding sessionId in object: ", sessionIdParseErr.Error())
							return nil
						}

						errMsg := fmt.Sprint("error while parsing sessionId from cached response: ", sessionIdParseErr.Error())
						return fmt.Errorf(errMsg)
					}

					var newSessionId string

					// If the value is not an object, it means that the cache was not updated
					// indicating that the OpenAI call had failed.
					if valueType != jsonparser.Object {
						// Delete the older sessionId since a new one will be injected
						// responseWithoutOlderSessionId := jsonparser.Delete(responseToWrite, string(key), querytranslate.SessionIDKeyToInject)

						updatedRSResponse, AIAnswerErr := querytranslate.ExecuteAIAnswerInQuery(rsAPIBody, responseToWrite, indices, openai.SessionInstance(), userId)
						if AIAnswerErr != nil {
							errMsg := ": error while executing AI Answer query: " + AIAnswerErr.Error()
							log.Warnln(logTag, errMsg)
							return fmt.Errorf(errMsg)
						}
						responseToWrite = updatedRSResponse

						// Extract the sessionId so that we can wait for it to resolve
						extractedSessionId, readErr := jsonparser.GetString(updatedRSResponse, string(key), querytranslate.SessionIDKeyToInject)
						if readErr != nil {
							errMsg := "sessionId not injected, cannot continue with cache hit: " + readErr.Error()
							log.Warnln(logTag, ": ", errMsg)
							return fmt.Errorf(errMsg)
						}

						newSessionId = extractedSessionId

						// // Update the cache with the new session ID and the updated
						// // response
						go func(key string, value []byte) {
							responseDetails := sessionMap.GetResponse(newSessionId)

							if responseDetails == nil {
								log.Warnln(logTag, ": not updating cache since response is nil!")
								return
							}

							for !responseDetails.GetIsReady() {
								time.Sleep(5 * time.Second)
								continue
							}

							// If the response failed then we don't need to update
							if responseDetails.GetIsFailed() {
								log.Debug(logTag, ": not updating cache since AI response failed!")
								return
							}

							// It should have resolved now

							// We can specify any maxDuration here since it will never actually
							// be used. The following call is explicitly to update an already existing
							// cache and thus we will not use the maxDuration.
							defaultMaxDuration := int64(0)

							// Calculate the item cost.
							cacheKey := cache.GetCacheKey(urlPath, out)
							// cost to store request (cache key)
							requestSize := int64(len([]byte(cacheKey)))
							// cost to store response
							responseSize := int64(len(value))
							itemCost := requestSize + responseSize

							cache.UpdateValueWithAIAnswer(cacheKey, key, responseDetails.Response(), responseDetails, value, defaultMaxDuration, itemCost)
						}(string(key), value)

					} else {
						// Generate new sessionId based on the older one
						var newSessionGenerateErr error
						newSessionId, newSessionGenerateErr = sessionMap.SessionFromBytes(sessionDetails, userId)
						if newSessionGenerateErr != nil {
							errMsg := fmt.Sprint("error while generating new session Id from details: ", newSessionGenerateErr.Error())
							log.Warnln(logTag, ": ", errMsg)
							return errors.New(errMsg)
						}
					}

					bodyWithNewSessionId, injectErr := jsonparser.Set(ValueAsByte, []byte(fmt.Sprintf(`"%s"`, newSessionId)), string(key), querytranslate.SessionIDKeyToInject)
					if injectErr != nil {
						errMsg := fmt.Sprint("error while injecting new sessionId: ", injectErr.Error())
						log.Warnln(logTag, ": ", errMsg)
						return fmt.Errorf(errMsg)
					}

					ValueAsByte = bodyWithNewSessionId
					return nil
				}
				return nil
			})

			if sessionIdUpdateErr != nil {
				errMsg := fmt.Sprint("error while updating sessionId in cached response: ", sessionIdUpdateErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				return nil, fmt.Errorf(errMsg)
			}
		}

		if rsAPIBody != nil {
			took := time.Since(*startTime).Milliseconds()

			// Get the older took value
			originalTook, _, _, err := jsonparser.Get(ValueAsByte, "settings", "took")
			if err == nil {
				// Calculate the performance save
				originalTookAsInt, tookToIntErr := strconv.Atoi(string(originalTook))
				if tookToIntErr != nil {
					log.Warnln(logTag, ": error while converting original took to int, ", tookToIntErr)
				} else {
					performanceSave = originalTookAsInt - int(took)
				}
			}

			// Modify `took` value for Cached responses
			responseBody, err := jsonparser.Set(ValueAsByte, []byte(fmt.Sprintf("%d", took)), "settings", "took")
			if err != nil {
				responseBody2, err2 := jsonparser.Set(ValueAsByte, []byte(fmt.Sprintf(`{ "took": %d }`, took)), "settings")
				if err2 != nil {
					log.Warnln(logTag, "unable to set settings.took key", err2)
				} else {
					responseToWrite = responseBody2
				}
			} else {
				responseToWrite = responseBody
			}

			// write `cached` key to settings object
			responseToWrite, err = jsonparser.Set(responseToWrite, []byte(fmt.Sprintf(`%t`, true)), "settings", "cached")
			if err != nil {
				responseToWrite, err = jsonparser.Set(responseToWrite, []byte(fmt.Sprintf(`{ "cached": %t }`, true)), "settings")
				if err != nil {
					log.Warnln(logTag, "unable to set settings.cached key", err)
				}
			}
		} else {
			responseToWrite = ValueAsByte
		}

		return &CachedResponse{
			Body:            responseToWrite,
			Headers:         headers,
			PerformanceSave: int64(performanceSave),
		}, nil
	}
	return nil, nil
}
