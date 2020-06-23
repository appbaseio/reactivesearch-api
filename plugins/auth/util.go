package auth

import (
	"sync"
)

// UserToPasswordCache represents a map of bcrypt validated users
var UserToPasswordCache = make(map[string]interface{})

// CurrentProcessMutex to stop concurrent writes on map
var CurrentProcessMutex = sync.RWMutex{}

// SavePassword saved the password in the cache
func SavePassword(username string, password string) {
	CurrentProcessMutex.Lock()
	defer CurrentProcessMutex.Unlock()
	UserToPasswordCache[username] = password
}

// ClearPassword clears the password in the cache
func ClearPassword(username string) {
	CurrentProcessMutex.Lock()
	defer CurrentProcessMutex.Unlock()
	delete(UserToPasswordCache, username)
}

// IsPasswordExist checks whether the password in the cache or not
func IsPasswordExist(username string, password string) bool {
	CurrentProcessMutex.Lock()
	defer CurrentProcessMutex.Unlock()
	cachedPassword, ok := UserToPasswordCache[username]
	if !ok {
		return false
	}
	return cachedPassword == password
}
