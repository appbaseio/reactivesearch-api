package analytics

import (
	"strings"
	"sync"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/util"
)

type item struct {
	value      string
	lastAccess int64
}
type TTLMap struct {
	m map[string]*item
	l sync.Mutex
}

func InitSession(ln int, maxTTL int) (m *TTLMap) {
	m = &TTLMap{m: make(map[string]*item, ln)}
	go func() {
		for now := range time.Tick(time.Second) {
			m.l.Lock()
			for k, v := range m.m {
				if now.Unix()-v.lastAccess > int64(maxTTL) {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
	return
}

func (m *TTLMap) Len() int {
	return len(m.m)
}

func (m *TTLMap) Put(k, v string) {
	m.l.Lock()
	it, ok := m.m[k]
	if !ok {
		it = &item{value: v}
		m.m[k] = it
	}
	it.lastAccess = time.Now().Unix()
	m.l.Unlock()
}

func (m *TTLMap) Get(k string) (v string) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = it.value
		it.lastAccess = time.Now().Unix()
	}
	m.l.Unlock()
	return
}

// Delete the stale sessions by user_id/ip
// at a time there should only be two keys present for a session ID
// For example, if query is `harry` then session map can only have same session ID for
// `harry` and `harr` to avoid issues if more than one char has been changed in the query

// TODO: Running a loop on active sessions is not performant
// It happens synchronously so it would affect the search latency
func (m *TTLMap) DeleteStaleSessions(whitelisted []string, userID string) {
	m.l.Lock()
	for k := range m.m {
		// at a time only two keys can have the same session ID
		if strings.HasPrefix(k, userID+separator) && !util.Contains(whitelisted, k) {
			delete(m.m, k)
		}
	}
	m.l.Unlock()
}
