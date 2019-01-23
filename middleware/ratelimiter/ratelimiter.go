package ratelimiter

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/util"
	"github.com/appbaseio-confidential/arc/util/iplookup"
	goredis "github.com/go-redis/redis"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/store/memory"
	"github.com/ulule/limiter/drivers/store/redis"
)

const (
	logTag          = "[ratelimiter]"
	defaultRedisDB  = 0
	defaultMaxRetry = 4
	redisAddr       = "accapi-staging.redis.cache.windows.net:6379"
	redisPassword   = "OJ5CsLWo+jxFWQ+XjNrT5smNSilvaQnSUkq8QVwMGR0="
)

var (
	instance *Ratelimiter
	once     sync.Once
)

// Ratelimiter limits the number of requests made by a permission per category
// as well as per IP. Creating direct instances of RateLimiter should be avoided.
// ratelimiter.Instance returns the singleton instance of the Ratelimiter.
type Ratelimiter struct {
	sync.Mutex
	limiters map[string]*limiter.Limiter
}

// Instance returns the singleton instance of ratelimiter.
func Instance() *Ratelimiter {
	once.Do(func() {
		instance = &Ratelimiter{
			limiters: make(map[string]*limiter.Limiter),
		}
	})
	return instance
}

func Limit() middleware.Middleware {
	return Instance().rateLimit
}

// RateLimit middleware limits the requests made to elasticsearch for each permission.
func (rl *Ratelimiter) rateLimit(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			remoteIP := iplookup.FromRequest(r)
			errMsg := "An error occurred while validating rate limit"
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			reqCategory, err := category.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			// limit on Categories per second
			categoryLimit, err := reqPermission.GetLimitFor(*reqCategory)
			if err != nil {
				util.WriteBackError(w, err.Error(), http.StatusUnauthorized)
				return
			}

			key := fmt.Sprintf("%s:%s", reqPermission.Username, *reqCategory)
			if rl.limitExceededByACL(key, categoryLimit) {
				util.WriteBackMessage(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// limit on IP per hour
			ipLimit := reqPermission.GetIPLimit()
			key = fmt.Sprintf("%s:%s", reqPermission.Username, remoteIP)
			if rl.limitExceededByIP(key, ipLimit) {
				util.WriteBackMessage(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}

		h(w, r)
	}
}

func (rl *Ratelimiter) limitExceededByACL(key string, aclLimit int64) bool {
	period := 1 * time.Second
	rem, _ := rl.peekLimit(key, aclLimit, period)
	if rem <= 0 {
		return true
	}
	rl.limit(key, aclLimit, period)
	return false
}

func (rl *Ratelimiter) limitExceededByIP(key string, ipLimit int64) bool {
	period := 1 * time.Hour
	rem, _ := rl.peekLimit(key, ipLimit, period)
	if rem <= 0 {
		return true
	}
	rl.limit(key, ipLimit, period)
	return false
}

func (rl *Ratelimiter) peekLimit(key string, limit int64, period time.Duration) (int64, bool) {
	l := rl.getLimiter(key, limit, period)
	if c, err := l.Peek(context.Background(), key); err == nil {
		return c.Remaining, c.Reached
	}
	// an error getting the limiter context ...
	return -1, false
}

func (rl *Ratelimiter) limit(key string, limit int64, period time.Duration) (int64, bool) {
	l := rl.getLimiter(key, limit, period)
	if c, err := l.Get(context.Background(), key); err == nil {
		return c.Remaining, c.Reached
	}
	// an error getting the limiter context ...
	return -1, false
}

func (rl *Ratelimiter) getLimiter(key string, limit int64, period time.Duration) *limiter.Limiter {
	rl.Lock()
	defer rl.Unlock()
	l, exists := rl.limiters[key]
	if !exists {
		l = rl.newLimiter(key, limit, period)
	}
	if l.Rate.Limit != limit {
		l.Rate.Limit = limit
	}
	return l
}

// A new instance for the given key is stored in the map each time this method is invoked.
// The access must be mediated by some kind of synchronization mechanism to prevent concurrent
// read/write operations to the map and vars.
func (rl *Ratelimiter) newLimiter(key string, limit int64, period time.Duration) *limiter.Limiter {
	store := memory.NewStore()
	rate := limiter.Rate{
		Limit:  limit,
		Period: period,
	}
	instance := limiter.New(store, rate)
	rl.limiters[key] = instance
	return instance
}

func (rl *Ratelimiter) newLimiterWithRedis(key string, limit int64, period time.Duration) *limiter.Limiter {
	option := &goredis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       defaultRedisDB,
	}
	client := goredis.NewClient(option)
	store, err := redis.NewStoreWithOptions(client, limiter.StoreOptions{
		Prefix:   key,
		MaxRetry: defaultMaxRetry,
	})
	if err != nil {
		log.Printf("%s: cannot create redis store for the rate limiter: %v", logTag, err)
		return nil
	}
	rate := limiter.Rate{
		Limit:  limit,
		Period: period,
	}
	instance := limiter.New(store, rate)
	rl.limiters[key] = instance
	return instance
}
