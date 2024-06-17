package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/model/index"
	"github.com/appbaseio-confidential/reactivesearch/plugins/openai"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/buger/jsonparser"
	"github.com/dgraph-io/ristretto"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

var searchCache *ristretto.Cache
var redisSearchCache *redis.Client
var useRedisCache bool = true

const CachedRequestHeader = "X-request-Cache"

var ctx = context.Background()

func GetSearchCache() *ristretto.Cache {
	return searchCache
}

func SanitizeRequest(req querytranslate.RSQuery) querytranslate.RSQuery {
	if req.Settings == nil {
		return req
	}
	sanitizeRequest := req

	// only cache `queryRule` and `enableQueryRules` because they can affect after query response
	sanitizeRequest.Settings = &querytranslate.Settings{
		EnableQueryRules: req.Settings.EnableQueryRules,
		QueryRule:        req.Settings.QueryRule,
	}

	return sanitizeRequest
}

func isRedisCacheEnabled() bool {
	cacheConfig := GetCachePreferences()
	if cacheConfig.Addr != nil && *cacheConfig.Addr != "" && useRedisCache {
		return true
	}
	return false
}

func initializeSearchCache() error {
	cacheConfig := GetCachePreferences()
	maxSize := int64(128)
	if cacheConfig.MaxSize != nil {
		maxSize = *cacheConfig.MaxSize
	}

	if cacheConfig.Addr != nil && *cacheConfig.Addr != "" {
		// Extract host and port using net.SplitHostPort
		host, port, err := net.SplitHostPort(*cacheConfig.Addr)
		if err != nil {
			if addrError, ok := err.(*net.AddrError); ok && addrError.Err == "missing port in address" {
				host = *cacheConfig.Addr // the original Addr is the host
				port = "6379"            // default Redis port
			} else {
				// Handle other potential errors
				useRedisCache = false
				log.Errorln(logTag, ", Error parsing Redis address:", err)
				return err
			}
		}

		// Join host and port using net.JoinHostPort
		address := net.JoinHostPort(host, port)
		var cachePassword string = ""
		var cacheDatabase int = 0
		if cacheConfig.Password != nil {
			cachePassword = *cacheConfig.Password
		}
		if cacheConfig.Database != nil {
			cacheDatabase = *cacheConfig.Database
		}
		// Initialize a new Redis client.
		redisSearchCache = redis.NewClient(&redis.Options{
			Addr:     address,
			Password: cachePassword,
			DB:       cacheDatabase,
		})
		// Test the connection to Redis.
		pong, err := redisSearchCache.Ping(ctx).Result()
		if err != nil {
			useRedisCache = false
			fmt.Println("Error connecting to Redis:", err)

			// NOTE: We are not returning the error here as we want
			// the ristretto cache to be initialized anyway
			// return err
		} else {
			useRedisCache = true
			log.Debugln(logTag, ": Redis connection established: ", pong)
		}
	}

	cacheInstance, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,                // number of keys to track frequency of (10M).
		MaxCost:     10000000 * maxSize, // maximum cost of cache (4GB).
		BufferItems: 64,                 // number of keys per Get buffer.
	})
	if err == nil {
		// Clear the memory if search cache already exist
		if searchCache != nil {
			searchCache.Clear()
		}
		searchCache = cacheInstance
	}

	return err
}

func writeToCache(url string, body string, value []byte, rsQuery *querytranslate.RSQuery) bool {
	cacheConfig := GetCachePreferences()
	var maxDuration int64 = 60
	if cacheConfig.MaxDuration != nil {
		maxDuration = *cacheConfig.MaxDuration
	}
	cacheKey := GetCacheKey(url, body)
	// cost to store request (cache key)
	requestSize := int64(len([]byte(cacheKey)))
	// cost to store response
	responseSize := int64(len(value))
	// overall cost to store a search query
	itemCost := requestSize + responseSize
	// init search cache if found nil
	if isRedisCacheEnabled() && redisSearchCache == nil {
		initializeSearchCache()
	} else if !isRedisCacheEnabled() && searchCache == nil {
		initializeSearchCache()
	}
	var isAdded bool
	if isRedisCacheEnabled() && redisSearchCache != nil {
		statusCmd := redisSearchCache.Set(ctx, cacheKey, string(value), time.Duration(time.Duration(maxDuration)*time.Second))
		if statusCmd.Err() != nil {
			log.Errorln(logTag, ": error while setting cache value: ", statusCmd.Err().Error())
			return isAdded
		} else {
			isAdded = true
		}
	} else {
		isAdded = searchCache.SetWithTTL(cacheKey, string(value), itemCost, time.Duration(time.Duration(maxDuration)*time.Second))
	}

	// If RS Query is nil, we don't need to do anything
	if rsQuery == nil {
		return isAdded
	}

	// Iterate the response to figure out the sessionId's and set them in
	// the map accordingly for later use.
	sessionMap := openai.SessionInstance()
	for _, queryEach := range rsQuery.Query {
		queryId := *queryEach.ID
		sessionIdValue, fetchErr := jsonparser.GetString(value, queryId, querytranslate.SessionIDKeyToInject)
		if fetchErr != nil {
			if fetchErr != jsonparser.KeyPathNotFoundError {
				log.Warnln(logTag, ": error while finding key for queryId: ", queryId, " with err: ", fetchErr.Error())
			}
			continue
		}

		responseDetails := sessionMap.GetResponse(sessionIdValue)

		// if `responseDetails` is not valid, throw an error
		if responseDetails == nil {
			log.Warnln(logTag, ": response returned for `sessionId` is nil: ", sessionIdValue)
			continue
		}

		// Wait for the ChatGPT response to resolve and then update the value here
		go func() {
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
			UpdateValueWithAIAnswer(cacheKey, queryId, responseDetails.Response(), responseDetails, value, maxDuration, itemCost)
		}()
	}

	return isAdded
}

func ReadResponseFromCache(url string, body string) interface{} {
	if redisSearchCache != nil {
		value, err := redisSearchCache.Get(ctx, GetCacheKey(url, body)).Result()
		if err == redis.Nil {
			return nil
		} else if err != nil {
			log.Errorln(logTag, ": error while reading cache value: ", err.Error())
			return nil
		} else {
			return value
		}
	} else {
		value, found := searchCache.Get(GetCacheKey(url, body))
		if !found {
			return nil
		}
		return value
	}
}

const cacheKeySeparator = "__"

// Cache key is prefixed by request url to avoid conflicts among same requests for different indices
func GetCacheKey(url string, body string) string {
	return url + cacheKeySeparator + body
}

type CacheConfig struct {
	EnableCache *bool `json:"enable_cache,omitempty"`
	// MaxSize represents the size limit for request cache
	// It should be in MB(s)
	MaxSize *int64 `json:"max_size,omitempty"`
	// MaxDuration represents the TTL(time to live) for a particular request
	// It should be in seconds
	MaxDuration *int64 `json:"max_duration,omitempty"`
	// Represents the indices to be cached
	Indices *[]string `json:"indices,omitempty"`
	// Represents external cache URL
	Addr *string `json:"addr,omitempty"`
	// Password
	Password *string `json:"password,omitempty"`
	// Database
	Database *int `json:"database,omitempty"`
}

// validates if incoming request's index(es) belong to allowed indices
func validateCacheIndices(req *http.Request, allowedIndexPatterns []string) bool {
	reqIndices, err := index.FromContext(req.Context())
	if err != nil {
		log.Errorln(logTag, ": cannot fetch indices from request context,", err)
		return false
	}
	for _, index := range reqIndices {
		for _, pattern := range *GetCachePreferences().Indices {
			matched, err := util.ValidateIndex(pattern, index)
			if err != nil {
				log.Errorln("invalid index regexp", pattern, "encountered: ", err)
				return false
			}
			if matched {
				return matched
			}
		}
	}
	return false
}

func ValidateCachePreferences(req *http.Request) bool {
	// Retrieve cache preferences
	cacheConfig := GetCachePreferences()
	// validate enable cache
	if cacheConfig.EnableCache != nil && *cacheConfig.EnableCache {
		return validateCacheIndices(req, *cacheConfig.Indices)
	}
	return false
}

// Apply response from cache
// 1. if `useCache` value is set to `true` (Highest priority, ignores the cache preference)
// 2. validate cache preferences
func ShouldApplyCache(req *http.Request, requestBody querytranslate.RSQuery) bool {
	// Don't cache _validate endpoint
	if util.IsRSAPIValidateRoute(req) {
		return false
	}
	if requestBody.Settings != nil && requestBody.Settings.UseCache != nil && !*requestBody.Settings.UseCache {
		return false
	}
	return ValidateCachePreferences(req)
}

// UpdateValueWithAIAnswer will inject the AIAnswer response into the cached
// response object
func UpdateValueWithAIAnswer(cacheKey string, queryId string, aiAnswerResponse []byte, response *openai.InternalChatGPTResponse, olderValue []byte, maxDuration int64, cost int64) {
	// Update the cache body of the above response if it was cached
	//
	// We will have to perform a few functions if the cache key is present
	// - get the ttl for the value
	// - delete the older value
	// - set a new value (updated response) with the older ttl

	// Update the response with AIAnswer key
	updatedValue, setErr := jsonparser.Set(olderValue, aiAnswerResponse, queryId, querytranslate.KeyToInject)
	if setErr != nil {
		log.Warnln(logTag, ": error while injecting the ChatGPT response into the older response: ", setErr.Error())
		return
	}

	// Don't remove the sessionId since we will need it when a cache hit
	// happens.

	// Inject the sessionDoc in bytes in order to use it later on
	// Sleep 5 seconds for the session doc to complete updating.
	time.Sleep(5 * time.Second)

	sessionDoc := response.GetSession()
	sessionInBytes, marshalErr := json.Marshal(sessionDoc)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshalling session doc to bytes: ", marshalErr.Error())
		return
	}

	updatedValue, setErr = jsonparser.Set(updatedValue, sessionInBytes, queryId, querytranslate.SessionIDKeyToInject)
	if setErr != nil {
		log.Warnln(logTag, ": error while injecting session in bytes into sessionID: ", setErr.Error())
		return
	}

	olderTTL, isFound := searchCache.GetTTL(cacheKey)
	if !isFound {
		olderTTL = time.Duration(time.Duration(maxDuration) * time.Second)
	}

	log.Debugln(logTag, ": setting the updated body into cache: ", string(updatedValue))
	if isRedisCacheEnabled() && redisSearchCache != nil {
		redisSearchCache.Set(ctx, cacheKey, string(updatedValue), olderTTL)
	} else {
		searchCache.SetWithTTL(cacheKey, string(updatedValue), cost, olderTTL)
	}
}
