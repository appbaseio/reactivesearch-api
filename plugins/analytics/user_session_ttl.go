package analytics

import (
	"sync"
	"time"
)

type ActiveUserSession struct {
	StartTime int64
	ID        string
	Bounce    bool
	TimeStamp string
}

type ActiveUserSessionItem struct {
	value      ActiveUserSession
	lastAccess int64
}
type ActiveUserSessionTTLMap struct {
	m map[string]*ActiveUserSessionItem
	l sync.Mutex
}

func InitUserSession(ln int, maxTTL int, initialValue *ActiveUserSessionTTLMap) (m *ActiveUserSessionTTLMap) {
	if initialValue != nil {
		m = initialValue
	} else {
		m = &ActiveUserSessionTTLMap{m: make(map[string]*ActiveUserSessionItem, ln)}
	}
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

func (m *ActiveUserSessionTTLMap) Len() int {
	return len(m.m)
}

func (m *ActiveUserSessionTTLMap) Put(k string, v ActiveUserSession) {
	m.l.Lock()
	it, ok := m.m[k]
	if !ok {
		it = &ActiveUserSessionItem{value: v}
		m.m[k] = it
	}
	it.lastAccess = time.Now().Unix()
	m.l.Unlock()
}

func (m *ActiveUserSessionTTLMap) Get(k string) (v *ActiveUserSession) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = &it.value
		it.lastAccess = time.Now().Unix()
	}
	m.l.Unlock()
	return
}

func (m *ActiveUserSessionTTLMap) Delete(k string) (v *ActiveUserSession) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = &it.value
		delete(m.m, k)
	}
	m.l.Unlock()
	return
}
