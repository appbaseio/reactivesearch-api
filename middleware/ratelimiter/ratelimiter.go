package ratelimiter

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/util"
	goredis "github.com/go-redis/redis"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/store/redis"
)

const (
	logTag          = "[ratelimiter]"
	DefaultRedisDB  = 0
	DefaultMaxRetry = 4
)

var (
	redisAddr     string
	redisPassword string
)

type RateLimiter struct {
	sync.Mutex
	limiters map[string]*limiter.Limiter
}

func New(storeAddr, storePassword string) *RateLimiter {
	redisAddr = storeAddr
	redisPassword = storePassword
	limiters := make(map[string]*limiter.Limiter)
	return &RateLimiter{
		limiters: limiters,
	}
}

func (rl *RateLimiter) RateLimit(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remoteIP := r.Header.Get("X-Forwarded-For")
		apiKey, _, ok := r.BasicAuth()
		if !ok {
			log.Printf("%s: user not logged in through basic auth.", logTag)
			return
		}

		p := r.Context().Value(permission.CtxKey)
		if p == nil {
			log.Printf("%s: unable to fetch user from request context", logTag)
			return
		}
		permissionObj, ok := p.(*permission.Permission)
		if !ok {
			log.Printf("%s: unable to cast to context user to user object: %v", logTag, p)
			return
		}

		a := r.Context().Value(acl.CtxKey)
		if a != nil {
			aclLimit := permissionObj.ACLLimit
			key := apiKey
			if rl.limitExceededByACL(key, aclLimit) {
				util.WriteBackMessage(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}

		ipLimit := permissionObj.IPLimit
		key := apiKey + remoteIP
		if rl.limitExceededByIP(key, ipLimit) {
			util.WriteBackMessage(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		h(w, r)
	}
}

func (rl *RateLimiter) limitExceededByACL(key string, aclLimit int64) bool {
	period := 1 * time.Second
	rem, _ := rl.peekLimit(key, aclLimit, period)
	if rem <= 0 {
		return true
	}
	reqRem, _ := rl.limit(key, aclLimit, period)
	log.Printf("%s: remaining 'acl_limit': %d", logTag, reqRem)
	return false
}

func (rl *RateLimiter) limitExceededByIP(key string, ipLimit int64) bool {
	// check ip limit
	period := 1 * time.Hour
	rem, _ := rl.peekLimit(key, ipLimit, period)
	if rem <= 0 {
		return true
	}
	reqRem, _ := rl.limit(key, ipLimit, period)
	log.Printf("%s: remaining 'ip_limit': %d", logTag, reqRem)
	return false
}

func (rl *RateLimiter) peekLimit(key string, limit int64, period time.Duration) (int64, bool) {
	l := rl.getLimiter(key, limit, period)
	if c, err := l.Peek(context.Background(), key); err == nil {
		return c.Remaining, c.Reached
	}
	// an error getting the limiter context ...
	return -1, false
}

func (rl *RateLimiter) limit(key string, limit int64, period time.Duration) (int64, bool) {
	l := rl.getLimiter(key, limit, period)
	if c, err := l.Get(context.Background(), key); err == nil {
		return c.Remaining, c.Reached
	}
	// an error getting the limiter context ...
	return -1, false
}

func (rl *RateLimiter) getLimiter(key string, limit int64, period time.Duration) *limiter.Limiter {
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
func (rl *RateLimiter) newLimiter(key string, limit int64, period time.Duration) *limiter.Limiter {
	option := &goredis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       DefaultRedisDB,
	}
	client := goredis.NewClient(option)
	store, err := redis.NewStoreWithOptions(client, limiter.StoreOptions{
		Prefix:   key,
		MaxRetry: DefaultMaxRetry,
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
