package cache

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
)

var cachePreferences CacheConfig

// To get cache preferences
func GetCachePreferences() CacheConfig {
	return cachePreferences
}

// To set cache preferences
func setCachePreferences(cacheConfig CacheConfig) error {
	err := validateCacheConfig(cacheConfig)
	if err != nil {
		log.Errorln(logTag, ": ", err)
		return err
	}
	if reflect.DeepEqual(cachePreferences, cacheConfig) {
		// if preferences haven't changed, don't re-init the cache
		return nil
	}
	log.Debugln(logTag, ": cache preferences have changed: ", pretty.Formatter(cachePreferences))
	cachePreferences = cacheConfig
	// only go further if cache is enabled in new preferences
	if cachePreferences.MaxSize != nil && cachePreferences.EnableCache != nil {
		// Re-initialize cache since we know that at least one setting has changed
		initializeSearchCache()
	}
	return nil
}

// To validate cache configuration
func validateCacheConfig(cacheConfig CacheConfig) error {
	if cacheConfig.MaxDuration != nil {
		// Minimum duration is 60s
		if *cacheConfig.MaxDuration < 60 {
			return fmt.Errorf("max_duration for cache can not be less than 60 seconds")
		}
		// Maximum duration is 24h(24*60*60 seconds)
		if *cacheConfig.MaxDuration > 24*60*60 {
			return fmt.Errorf("max_duration for cache can not be greater than 86,400 seconds (i.e. 24 hours)")
		}
	} else {
		return fmt.Errorf("max_duration for cache can't be nil, should be between [60, 86400] seconds: input is - %s", pretty.Formatter(cacheConfig))
	}
	if cacheConfig.MaxSize != nil {
		if *cacheConfig.MaxSize < 128 {
			return fmt.Errorf("max_size for cache can not be less than 128 MB")
		}
		// Maximum size can't be less than 4GB
		if *cacheConfig.MaxSize > (4 * 1000) {
			return fmt.Errorf("max_size for cache can not be greater than 4 GB")
		}
	} else {
		return fmt.Errorf("max_size for cache can't be nil, should be between [128MB, 4GB]")
	}
	if cacheConfig.Addr != nil {
		// Split the Redis Address by ":" to separate host and port.
		parts := strings.Split(*cacheConfig.Addr, ":")

		switch len(parts) {
		case 1: // Only host is present, append the default port.
			return nil
		case 2: // Both host and port are present.
			// Validate the port.
			port, err := strconv.Atoi(parts[1])
			if err != nil {
				return fmt.Errorf("invalid port: %s", parts[1])
			}
			// Port should be between 1 and 65535.
			if port < 1 || port > 65535 {
				return fmt.Errorf("port out of range: %d", port)
			}
		default:
			return fmt.Errorf("invalid Redis Address format: %s", *cacheConfig.Addr)
		}
	}
	return nil
}
