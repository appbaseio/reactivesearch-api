package auth

import (
	"sync"
)

// UserToPasswordCache represents a map of bcrypt validated users by domain/tenantId
var UserToPasswordCache = make(map[string]map[string]string)

// CurrentProcessMutex to stop concurrent writes on map
var CurrentProcessMutex = sync.RWMutex{}

// SavePassword saved the password in the cache
func SavePassword(tenant string, username string, password string) {
	CurrentProcessMutex.Lock()
	if UserToPasswordCache[tenant] == nil {
		UserToPasswordCache[tenant] = map[string]string{
			username: password,
		}
	} else {
		UserToPasswordCache[tenant][username] = password
	}
	CurrentProcessMutex.Unlock()
}

// ClearPassword clears the password in the cache
func ClearPassword(tenant string, username string) {
	CurrentProcessMutex.Lock()
	if UserToPasswordCache[tenant] != nil {
		delete(UserToPasswordCache[tenant], username)
	}
	CurrentProcessMutex.Unlock()
}

// IsPasswordExist checks whether the password in the cache or not
func IsPasswordExist(tenant string, username string, password string) bool {
	CurrentProcessMutex.Lock()
	defer CurrentProcessMutex.Unlock()
	if cachedDomain, ok := UserToPasswordCache[tenant]; ok {
		cachedPassword, ok := cachedDomain[username]
		if !ok {
			return false
		}
		return cachedPassword == password
	}
	return false
}

// deletes the user record from local state
func ClearLocalUser(tenant string, username string) {
	// Clear username record from the cache
	ClearPassword(tenant, username)
	// Clear user record from the user cache
	RemoveCredentialFromCache(tenant, username)
}
