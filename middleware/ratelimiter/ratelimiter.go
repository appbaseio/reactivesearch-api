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
	//goredis "github.com/go-redis/redis"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/store/memory"
	//"github.com/ulule/limiter/drivers/store/redis"
)

const (
	logTag          = "[ratelimiter]"
	defaultRedisDB  = 0
	defaultMaxRetry = 4
	redisAddr       = "accapi-staging.redis.cache.windows.net:6379"
	redisPassword   = "OJ5CsLWo+jxFWQ+XjNrT5smNSilvaQnSUkq8QVwMGR0="
)

var (
	instance *ratelimiter
	once     sync.Once
)

type ratelimiter struct {
	sync.Mutex
	limiters map[string]*limiter.Limiter
}

func Instance() *ratelimiter {
	once.Do(func() {
		instance = &ratelimiter{
			limiters: make(map[string]*limiter.Limiter),
		}
	})
	return instance
}

func (rl *ratelimiter) RateLimit(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remoteIP := r.Header.Get("X-Forwarded-For")
		userId, _, ok := r.BasicAuth()
		if !ok {
			log.Printf("%s: user not logged in through basic auth.", logTag)
			return
		}

		ctxPermission := r.Context().Value(permission.CtxKey)
		if ctxPermission == nil {
			log.Printf("%s: unable to fetch permission from request context", logTag)
			return
		}
		obj, ok := ctxPermission.(*permission.Permission)
		if !ok {
			log.Printf("%s: unable to cast context permission to *permission.Permission: %v",
				logTag, ctxPermission)
			return
		}

		ctxACL := r.Context().Value(acl.CtxKey)
		if ctxACL == nil {
			log.Printf("%s: unable to fetch acl from request context", logTag)
			return
		}
		aclObj, ok := ctxACL.(*acl.ACL)
		if !ok {
			log.Printf("%s: unable to cast context acl to *acl.ACL: %v", logTag, ctxACL)
			return
		}

		aclLimit := obj.GetLimitFor(*aclObj)
		log.Printf("%s: aclLimit=%d", logTag, aclLimit)
		key := userId + aclObj.String() // limit per acl
		if rl.limitExceededByACL(key, aclLimit) {
			util.WriteBackMessage(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		ipLimit := obj.Limits.IPLimit
		key = userId + remoteIP
		if rl.limitExceededByIP(key, ipLimit) {
			util.WriteBackMessage(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		h(w, r)
	}
}

func (rl *ratelimiter) limitExceededByACL(key string, aclLimit int64) bool {
	period := 1 * time.Second
	rem, _ := rl.peekLimit(key, aclLimit, period)
	if rem <= 0 {
		return true
	}
	reqRem, _ := rl.limit(key, aclLimit, period)
	log.Printf("%s: remaining 'acl_limit': %d", logTag, reqRem)
	return false
}

func (rl *ratelimiter) limitExceededByIP(key string, ipLimit int64) bool {
	period := 1 * time.Hour
	rem, _ := rl.peekLimit(key, ipLimit, period)
	if rem <= 0 {
		return true
	}
	reqRem, _ := rl.limit(key, ipLimit, period)
	log.Printf("%s: remaining 'ip_limit': %d", logTag, reqRem)
	return false
}

func (rl *ratelimiter) peekLimit(key string, limit int64, period time.Duration) (int64, bool) {
	l := rl.getLimiter(key, limit, period)
	if c, err := l.Peek(context.Background(), key); err == nil {
		return c.Remaining, c.Reached
	}
	// an error getting the limiter context ...
	return -1, false
}

func (rl *ratelimiter) limit(key string, limit int64, period time.Duration) (int64, bool) {
	l := rl.getLimiter(key, limit, period)
	if c, err := l.Get(context.Background(), key); err == nil {
		return c.Remaining, c.Reached
	}
	// an error getting the limiter context ...
	return -1, false
}

func (rl *ratelimiter) getLimiter(key string, limit int64, period time.Duration) *limiter.Limiter {
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
func (rl *ratelimiter) newLimiter(key string, limit int64, period time.Duration) *limiter.Limiter {
	//option := &goredis.Options{
	//	Addr:     redisAddr,
	//	Password: redisPassword,
	//	DB:       DefaultRedisDB,
	//}
	//client := goredis.NewClient(option)
	//store, err := redis.NewStoreWithOptions(client, limiter.StoreOptions{
	//	Prefix:   key,
	//	MaxRetry: DefaultMaxRetry,
	//})
	//if err != nil {
	//	log.Printf("%s: cannot create redis store for the rate limiter: %v", logTag, err)
	//	return nil
	//}
	store := memory.NewStore()
	rate := limiter.Rate{
		Limit:  limit,
		Period: period,
	}
	instance := limiter.New(store, rate)
	rl.limiters[key] = instance
	return instance
}
