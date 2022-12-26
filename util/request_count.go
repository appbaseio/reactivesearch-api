package util

import (
	"sync"

	"github.com/robfig/cron"
)

// RequestCounter will count the requests
type RequestCounter struct {
	Value        int
	allowedValue int
	resetJob     *cron.Cron
	writeMutex   *sync.Mutex
	isLimit      bool
}

// NewRequestCounter will create a new reset counter that will
// be initialized with 0
func NewRequestCounter() *RequestCounter {
	return &RequestCounter{Value: 0, allowedValue: 0, isLimit: true}
}

// Reset will reset the counter
func (r *RequestCounter) Reset() {
	r.writeMutex.Lock()
	defer r.writeMutex.Unlock()
	r.Value = 0
}

// Increment will increment the counter by 1
func (r *RequestCounter) Increment() {
	r.writeMutex.Lock()
	defer r.writeMutex.Unlock()
	r.Value += 1
}

// IsExceeded will return if the value has exceeded the allowed value
func (r *RequestCounter) IsExceeded() bool {
	return r.isLimit && r.Value > r.allowedValue
}

// SetLimit will set the limit for the counter
func (r *RequestCounter) SetLimit(value int) {
	r.allowedValue = value
	r.isLimit = true
}

// SetNoLimit will set the no-limit flag as true for the counter
func (r *RequestCounter) SetNoLimit() {
	r.isLimit = false
}

// SetResetInterval will set the interval for resetting the counter
// using a cronjob
func (r *RequestCounter) SetResetInterval(interval string) error {
	resetJob := cron.New()
	jobInitErr := resetJob.AddFunc(interval, func() {
		// Reset the counter
		r.Reset()
	})

	if jobInitErr != nil {
		return jobInitErr
	}

	resetJob.Start()
	r.resetJob = resetJob
	return nil
}

// TenantRequestCount will store the requests of the tenant
type TenantRequestCount struct {
	countPerMin *RequestCounter
}

// NewTenantRequestCount will return a new tenant request count that
// will be initialized with two cronjobs.
// - Cronjob that runs every minute and resets the counter to 0
// - Cronjob that runs every hour and resets the count to 0
func NewTenantRequestCount() *TenantRequestCount {
	perMinCounter := NewRequestCounter()
	perMinCounter.SetResetInterval("@every 1m")

	return &TenantRequestCount{
		countPerMin: perMinCounter,
	}
}

// Increment will increase the counter for both per-min and per-hour
func (t *TenantRequestCount) Increment() {
	t.countPerMin.Increment()
}

// IsExceeded will check if any counter has exceeded the limit
// allowed
func (t *TenantRequestCount) IsExceeded() bool {
	return t.countPerMin.IsExceeded()
}

// SetLimit will set the limit based on the passed plan
func (t *TenantRequestCount) SetLimit(plan *Plan) {
	// Fetch the limits based on the plan
	requestLimit := plan.LimitForPlan().Requests

	if requestLimit.NoLimit {
		t.countPerMin.SetNoLimit()
		return
	}

	t.countPerMin.SetLimit(requestLimit.Value)
}

// tenantToRequestsMap will contain the request count on a per tenant
// basis.
var tenantToRequestsMap = make(map[string]*TenantRequestCount)

// InitRequestMap will initialize the request map counter for all tenants
func InitRequestMap() {
	slsInstances := GetSLSInstances()

	// Iterate over all the valid SLS instances and add their counter
	for _, instanceDetails := range slsInstances {
		// Don't add the instances that are already present because
		// this way we won't reset the counter

		// TODO: Handle situations where the plan is updated
		_, exists := tenantToRequestsMap[instanceDetails.TenantID]
		if exists {
			continue
		}

		newTR := NewTenantRequestCount()
		newTR.SetLimit(instanceDetails.Tier)
		tenantToRequestsMap[instanceDetails.TenantID] = newTR
	}
}

// GetRequestCounterForTenant will get the request counter for
// the passed tenantID.
//
// If it doesn't exist, we return a new counter
func GetRequestCounterForTenant(tenantID string) *TenantRequestCount {
	requestCounter, exists := tenantToRequestsMap[tenantID]
	if !exists {
		return NewTenantRequestCount()
	}
	return requestCounter
}
