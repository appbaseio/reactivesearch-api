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
	CurrentProcessMutex.Lock()
	cachedPassword, ok := UserToPasswordCache[username]
	CurrentProcessMutex.Unlock()
	if !ok {
		return false
	}
	return cachedPassword == password
}

// deletes the user record from local state
func ClearLocalUser(username string) {
	// Clear username record from the cache
	ClearPassword(username)
	// Clear user record from the user cache
	RemoveCredentialFromCache(username)
}
