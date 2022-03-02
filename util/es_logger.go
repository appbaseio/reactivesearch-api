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
	log.Debugln("[ElasticSearch: Trace] => ", fmt.Sprintf(format, vars...))
}

type WrapKitLoggerError struct {
	log.Logger
}

func (logger WrapKitLoggerError) Printf(format string, vars ...interface{}) {
	// If the log contains deprecation, print it as debug and return
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
		log.Debug("[ElasticSearch: Trace] => ", formattedStr)
		return true
	}

	return false
}
