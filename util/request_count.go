package util

import (
	"sync"

	"github.com/robfig/cron"
)

// RequestCounter will count the requests
type RequestCounter struct {
	Value      int
	resetJob   *cron.Cron
	writeMutex *sync.Mutex
}

// NewRequestCounter will create a new reset counter that will
// be initialized with 0
func NewRequestCounter() *RequestCounter {
	return &RequestCounter{Value: 0}
}

// Reset will reset the counter
func (r *RequestCounter) Reset() {
	r.writeMutex.Lock()
	defer r.writeMutex.Unlock()
	r.Value = 0
}

// SetResetInterval will set the interval for resetting the cronjob
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
	countPerMin  *RequestCounter
	countPerHour *RequestCounter
}

// NewTenantRequestCount will return a new tenant request count that
// will be initialized with two cronjobs.
// - Cronjob that runs every minute and resets the counter to 0
// - Cronjob that runs every hour and resets the count to 0
func NewTenantRequestCount() *TenantRequestCount {
	perMinCounter := NewRequestCounter()
	perMinCounter.SetResetInterval("@every 1m")

	perHourCounter := NewRequestCounter()
	perHourCounter.SetResetInterval("@every 1h")

	return &TenantRequestCount{
		countPerMin:  perMinCounter,
		countPerHour: perHourCounter,
	}
}

// tenantToRequestsMap will contain the request count on a per tenant
// basis.
var tenantToRequestsMap = make(map[string]*TenantRequestCount)
