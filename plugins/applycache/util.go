package applycache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/plugins/openai"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

type CacheStatES struct {
	Day               int64 `json:"day"`
	CacheRequestCount int64 `json:"cache_request_count"`
	CacheHit          int64 `json:"cache_hit"`
	CacheMiss         int64 `json:"cache_miss"`
	PerformanceSave   int64 `json:"performance_save"`
}

// getDateAtMidnight will return the date of the current
// day at midnight as an unix timestamp
//
// The time is calculated at the current hour and then the
// 24 hour is truncated from it to get the time at midnight.
func getDateAtMidnight() int64 {
	return time.Now().Truncate(24 * time.Hour).Unix()
}

// generateDayID will generate the unique ID for the date
func generateDayID() string {
	return fmt.Sprint(getDateAtMidnight())
}

type RollOverStat struct {
	Stat *CacheStatES
	mu   sync.Mutex
}

// Init will initiate the RollOverStat value and return
// a new instance
func (r *RollOverStat) Init() *RollOverStat {
	r.Stat = &CacheStatES{
		Day:               0,
		CacheRequestCount: 0,
		CacheHit:          0,
		CacheMiss:         0,
		PerformanceSave:   0,
	}
	return r
}

// Rollover will roll over the stats present currently
// and create an empty container to keep new data.
//
// This function will not take care of rolling over on every
// n minutes or hours. That should be handled at a top level.
func (r *RollOverStat) RollOver() *CacheStatES {
	r.mu.Lock()
	currentStat := r.Stat

	r.Stat = new(CacheStatES)
	r.mu.Unlock()

	return currentStat
}

// Add will add new stats to the current stat object.
//
// The value of performanceSave will be ignored if the cache
// was not hit.
func (r *RollOverStat) Add(isHit bool, performanceSave int64) {
	if r == nil || r.Stat == nil {
		return
	}
	r.mu.Lock()

	r.Stat.CacheRequestCount += 1

	// If not a hit then ignore the value of performanceSave
	if isHit {
		r.Stat.CacheHit += 1
		r.Stat.PerformanceSave += performanceSave
	} else {
		r.Stat.CacheMiss += 1
	}
	r.mu.Unlock()
}

// RollOverJob will rollover the stats every 10 minutes.
//
// This function will use the generatedID and use the rolled
// over stats every 10 minutes.
// This function assumes that the document is already present
// in the index with a proper day value set in the document so
// rollover will only update the stat values.
func (c *Cache) RollOverJob() {
	dayID := generateDayID()

	tenMinStats := c.rollOver.RollOver()

	updateScript := `
	ctx._source.cache_request_count += params.requestCount;
	ctx._source.cache_hit += params.hit;
	ctx._source.cache_miss += params.miss;
	ctx._source.performance_save += params.performanceSave;
	`

	updateParams := map[string]interface{}{
		"hit":             tenMinStats.CacheHit,
		"miss":            tenMinStats.CacheMiss,
		"performanceSave": tenMinStats.PerformanceSave,
		"requestCount":    tenMinStats.CacheRequestCount,
	}

	updateErr := c.es.updateStatScript(context.Background(), updateScript, updateParams, dayID)
	if updateErr != nil {
		log.Warnln(logTag, ": error while updating ten min stats in index, ", updateErr)
	}

}

// CreateDayStat will create the stat for the day using the dayID
// generated.
//
// This function should run at 12 AM everyday and will create a
// stat object with 0 values initialized.
func (c *Cache) CreateDayStat() {
	date := getDateAtMidnight()

	initStat := CacheStatES{
		Day:               date,
		CacheRequestCount: 0,
		CacheHit:          0,
		CacheMiss:         0,
		PerformanceSave:   0,
	}

	err := c.es.addStat(context.Background(), initStat)
	if err != nil {
		log.Errorln(logTag, ": error while creating initial day document in index, ", err)
		return
	}
}

// DeleteOlderRecords will delete all records that are older than 30 days.
//
// This function will run at midnight everyday and will delete all records
// that are less than the created unix timestamp.
func (c *Cache) DeleteOlderRecords() {
	olderDate := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -30).Unix()

	c.es.deleteOlderRecordsByDate(context.Background(), olderDate)
}

// StartJobs will start the cronjobs for applying cache stats
//
// This function will start two jobs:
// - run every day at 12 AM and create document for day
// - run every 10 mins and roll over stats
// - run delete older record job at 12 AM every day
func (c *Cache) StartJobs() {
	// Create the day job for 12 AM
	createDayStatJob := cron.New()
	createDayStatJob.AddFunc("@midnight", c.CreateDayStat)
	createDayStatJob.Start()

	// There might be a case when the server starts after midnight and in
	// such a case the document will not be present.
	// This will be a problem since the following job will run every 10 mins
	// and try to update the document.
	// As a failsafe, we will call the createDayStat function on startup as
	// well.
	//
	// This call will not affect if the server does start at midnight because
	// the same empty document will be created again.
	//
	// We also need to check if the stat for the day is already present and only if
	// it's not present, we need to create the stat for the day.
	isExists, idCheckErr := c.es.isIdExists(context.Background(), generateDayID())
	if idCheckErr != nil {
		log.Warnln(logTag, "error occurred while checking if the ID exists in the index, ", idCheckErr)
		c.CreateDayStat()
	}

	// If doesn't exist, create it.
	if !isExists {
		log.Info(logTag, ": creating document for current day since none exists!")
		c.CreateDayStat()
	}

	// Add job for every 10 mins
	updateTenMinStatsJob := cron.New()
	updateTenMinStatsJob.AddFunc("@every 10m", c.RollOverJob)
	updateTenMinStatsJob.Start()

	// Add job to delete older records
	deleteOldRecordJob := cron.New()
	deleteOldRecordJob.AddFunc("@midnight", c.DeleteOlderRecords)
	deleteOldRecordJob.Start()
}

// parseStringToUnix will parse the string time to unix integer.
//
// This method is made to extract the `from` and `to` values to
// integer instead of string.
func parseStringToUnix(timeAsString string) (int64, error) {
	parsedAsTime, parseErr := time.Parse(time.RFC3339, timeAsString)

	if parseErr != nil {
		return 0, parseErr
	}

	return parsedAsTime.Unix(), nil
}

// parseTimeString will parse the passed time string to
// make it in the format dd/mm/yyyy.
//
// The passed string should be in the format yyyy-mm-ddT*
func parseTimeString(passedTime string) string {
	timeArr := strings.Split(strings.Split(passedTime, "T")[0], "-")

	// timeArr should be of length 3
	// We need to reverse this array
	// and join it with /
	for i, j := 0, len(timeArr)-1; i < j; i, j = i+1, j-1 {
		timeArr[i], timeArr[j] = timeArr[j], timeArr[i]
	}

	return strings.Join(timeArr, "/")
}

// IsOpenAIEnabled will check if OpenAI is enabled by checking
// a few things.
// - If tier allows OpenAI
// - If cluster has OpenAI enabled
// - If passed index is whitelisted
func IsOpenAIEnabled(indices []string) bool {
	// Check if cluster's plan is one of the valid OpenAI plans
	// or the open AI flag is enabled explicitly.
	if !openai.IsOpenAIAllowed() {
		return false
	}

	openAIInstance := openai.Instance()

	// Check if user has OpenAI enabled
	if !openAIInstance.IsOpenAIEnabled() {
		return false
	}

	// Check whether or not all the indexes in the list are
	// whitelisted for allowing OpenAI
	areAllIndexesWhitelisted := true
	for _, index := range indices {
		if !openAIInstance.GetConfig().IsIndexWhitelisted(index) {
			areAllIndexesWhitelisted = false
			continue
		}
	}

	return areAllIndexesWhitelisted
}
