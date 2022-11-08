package util

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

type WrapKitLoggerDebug struct {
	log.Logger
}

func (logger WrapKitLoggerDebug) Printf(format string, vars ...interface{}) {
	cleanSenstiveData(&vars)
	log.Debugln("[ElasticSearch: Trace] => ", fmt.Sprintf(format, vars...))
}

type WrapKitLoggerError struct {
	log.Logger
}

func (logger WrapKitLoggerError) Printf(format string, vars ...interface{}) {
	cleanSenstiveData(&vars)

	formattedStr := fmt.Sprintf(format, vars...)
	if DebugDeprecationWarns(formattedStr) {
		return
	}

	log.Errorln("[ElasticSearch: Error] => ", formattedStr)
}

// DebugDeprecationWarns converts all the error logs containing
// deprecation warnings to debug logs so that it doesn't invoke sentry
func DebugDeprecationWarns(formattedStr string) bool {
	// Check if any of the vars contain `deprecation` in it.
	isDeprecated, _ := regexp.MatchString(`.*deprecation.*`, strings.ToLower(formattedStr))

	if isDeprecated {
		log.Debugln("[ElasticSearch: Trace] => ", formattedStr)
		return true
	}

	return false
}

// cleanSenstiveData cleans credentials from the
// variables, if any.
func cleanSenstiveData(vars *[]interface{}) {
	// Check if any var contains an URL, if it does, replace auth from the URL
	for index, passedVar := range *vars {
		// Cast the interface to a string
		stringedVar, ok := passedVar.(string)
		if !ok {
			continue
		}

		cleanedVar := CleanPasswordFromURL(stringedVar)

		(*vars)[index] = cleanedVar
	}
}

// CleanPasswordFromURL will clean password from the URL if
// it is present
func CleanPasswordFromURL(URL string) string {
	// Check if URL
	isURL, _ := regexp.MatchString(`^https?://(www.)?.+\..+$`, URL)
	if !isURL {
		return URL
	}

	// If it is an URL, clean it up
	cleanerRe := regexp.MustCompile(`\/\/(?P<username>.+):.+@`)
	cleanedVar := cleanerRe.ReplaceAllString(URL, "//${username}:********@")
	return cleanedVar
}
