package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/plugins/openai"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/buger/jsonparser"
	"github.com/dgraph-io/ristretto"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
)

var (
	searchCache      *ristretto.Cache
	redisSearchCache *redis.Client
	useRedisCache    bool = true
	cacheInitOnce    sync.Once
	ctx              = context.Background()
)

const CachedRequestHeader = "X-request-Cache"

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

	// Initialize in-memory cache regardless of Redis cache
	cacheInstance, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,                   // number of keys to track frequency of (10M).
		MaxCost:     maxSize * 1024 * 1024, // maximum cost of cache in bytes, maxSize is in MB.
		BufferItems: 64,                    // number of keys per Get buffer.
	})
	if err == nil {
		// Clear the memory if search cache already exist
		if searchCache != nil {
			searchCache.Clear()
		}
		searchCache = cacheInstance
	} else {
		log.Errorln(logTag, ": error initializing in-memory cache: ", err)
		// If in-memory cache cannot be initialized, set searchCache to nil
		searchCache = nil
	}

	// Attempt to initialize Redis cache if configured
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
				redisSearchCache = nil
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
			redisSearchCache = nil
			return err
		} else {
			useRedisCache = true
			log.Debugln(logTag, ": Redis connection established: ", pong)
		}
	} else {
		useRedisCache = false
		redisSearchCache = nil
	}

	return nil
}

func writeToCache(url string, body []byte, value []byte, rsQuery *querytranslate.RSQuery) bool {
	// Ensure cache is initialized only once
	cacheInitOnce.Do(func() {
		initializeSearchCache()
	})
	cacheConfig := GetCachePreferences()
	var maxDuration int64 = 60
	if cacheConfig.MaxDuration != nil {
		maxDuration = *cacheConfig.MaxDuration
	}
	cacheKey := GetCacheKey(url, body)
	// cost to store request (cache key)
	requestSize := int64(len(cacheKey))
	// cost to store response
	responseSize := int64(len(value))
	// overall cost to store a search query
	itemCost := requestSize + responseSize
	var isAdded bool
	if isRedisCacheEnabled() && redisSearchCache != nil {
		statusCmd := redisSearchCache.Set(ctx, cacheKey, value, time.Duration(maxDuration)*time.Second)
		if statusCmd.Err() != nil {
			log.Errorln(logTag, ": error while setting cache value: ", statusCmd.Err().Error())
			return isAdded
		} else {
			isAdded = true
		}
	} else if searchCache != nil {
		isAdded = searchCache.SetWithTTL(cacheKey, value, itemCost, time.Duration(maxDuration)*time.Second)
	} else {
		log.Errorln(logTag, ": no cache initialized")
		return false
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

		// Passing queryId and responseDetails as arguments to the goroutine ensures that each goroutine operates on the correct data.
		go func(queryId string, responseDetails *openai.InternalChatGPTResponse) {
			for !responseDetails.GetIsReady() {
				time.Sleep(5 * time.Second)
				continue
			}

			if responseDetails.GetIsFailed() {
				log.Debug(logTag, ": not updating cache since AI response failed!")
				return
			}

			// Update cache with AI answer
			UpdateValueWithAIAnswer(cacheKey, queryId, responseDetails.Response(), responseDetails, value, maxDuration, itemCost)
		}(queryId, responseDetails)
	}

	return isAdded
}

func ReadResponseFromCache(url string, body []byte) (string, []byte) {
	// Ensure cache is initialized only once
	cacheInitOnce.Do(func() {
		initializeSearchCache()
	})
	cacheKey := GetCacheKey(url, body)
	if isRedisCacheEnabled() && redisSearchCache != nil {
		value, err := redisSearchCache.Get(ctx, cacheKey).Bytes()
		if err == redis.Nil {
			return cacheKey, nil
		} else if err != nil {
			log.Errorln(logTag, ": error while reading cache value: ", err.Error())
			return cacheKey, nil
		} else {
			return cacheKey, value
		}
	} else {
		value, found := searchCache.Get(cacheKey)
		if !found {
			return cacheKey, nil
		}
		valueBytes, ok := value.([]byte)
		if !ok {
			log.Errorln(logTag, ": unexpected type in cache")
			return cacheKey, nil
		}
		return cacheKey, valueBytes
	}
}

const cacheKeySeparator = "__"

// Cache key is prefixed by request url to avoid conflicts among same requests for different indices
func GetCacheKey(url string, body []byte) string {
	hasher := sha256.New()
	hasher.Write([]byte(url))
	hasher.Write([]byte(cacheKeySeparator))
	hasher.Write(body)
	return hex.EncodeToString(hasher.Sum(nil))
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
	// Poll for the session document's readiness with a timeout. This prevents unnecessary delays & provides a more reliable and efficient way to wait for the session document.
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

outerLoop:
	for {
		select {
		case <-timeout:
			log.Warnln(logTag, ": timeout waiting for session document to be ready")
			return
		case <-ticker.C:
			sessionDoc := response.GetSession()
			if sessionDoc != nil {
				// Proceed with updating the cache
				break outerLoop
			}
		}
	}

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

	var olderTTL time.Duration
	if isRedisCacheEnabled() && redisSearchCache != nil {
		ttlCmd := redisSearchCache.TTL(ctx, cacheKey)
		ttl, err := ttlCmd.Result()
		if err != nil {
			log.Warnln(logTag, ": error getting TTL from Redis:", err)
			olderTTL = time.Duration(maxDuration) * time.Second
		} else {
			olderTTL = ttl
			if olderTTL <= 0 {
				olderTTL = time.Duration(maxDuration) * time.Second
			}
		}
	} else if searchCache != nil {
		var ok bool
		olderTTL, ok = searchCache.GetTTL(cacheKey)
		if !ok {
			olderTTL = time.Duration(maxDuration) * time.Second
		}
	} else {
		log.Errorln(logTag, ": no cache initialized")
		return
	}

	log.Debugln(logTag, ": setting the updated body into cache: ", string(updatedValue))
	if isRedisCacheEnabled() && redisSearchCache != nil {
		redisSearchCache.Set(ctx, cacheKey, updatedValue, olderTTL)
	} else {
		searchCache.SetWithTTL(cacheKey, updatedValue, cost, olderTTL)
	}
}
