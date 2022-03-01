package util

import (
	"fmt"
	"regexp"

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
	log.Errorln("[ElasticSearch: Error] => ", fmt.Sprintf(format, vars...))
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

		// Check if URL
		isURL, _ := regexp.MatchString(`^https?://(www.)?.+\..+$`, stringedVar)
		if !isURL {
			continue
		}

		// If it is an URL, clean it up
		cleanerRe := regexp.MustCompile(`//.+:.+@`)
		cleanedVar := cleanerRe.ReplaceAllString(stringedVar, "//***:***@")

		(*vars)[index] = cleanedVar
	}
}
