package elasticsearch

import "net/http"

// WhitelistedRoute will contain the path
// of the route
type WhitelistedRoute struct {
	Path string
}

// GetWhitelistedRoutesForSystem will return a map of path
// to the whitelisted methods allowed for that path
func GetWhitelistedRoutesForSystem() map[string][]string {
	return map[string][]string{
		"/{index}": {
			http.MethodGet, http.MethodPut,
		},
	}
}

// GetMethods will get the methods for the attached
// path.
func (w *WhitelistedRoute) GetMethods() []string {
	methods, exists := GetWhitelistedRoutesForSystem()[w.Path]
	if !exists {
		return make([]string, 0)
	}

	return methods
}

// IsMethodWhitelisted will check if the method is whitelisted
// for the path
func (w *WhitelistedRoute) IsMethodWhitelisted(methodPassed string) bool {
	for _, method := range w.GetMethods() {
		if methodPassed == method {
			return true
		}
	}

	return false
}
