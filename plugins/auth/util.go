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
	UserToPasswordCache[username] = password
	CurrentProcessMutex.Unlock()
}

// ClearPassword clears the password in the cache
func ClearPassword(username string) {
	CurrentProcessMutex.Lock()
	delete(UserToPasswordCache, username)
	CurrentProcessMutex.Unlock()
}

// IsPasswordExist checks whether the password in the cache or not
func IsPasswordExist(username string, password string) bool {
	cachedPassword, ok := UserToPasswordCache[username]
	if !ok {
		return false
	}
	return cachedPassword == password
}
