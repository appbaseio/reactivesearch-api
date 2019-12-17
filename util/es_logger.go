package util

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type wrapKitLoggerDebug struct {
	log.Logger
}

func (logger wrapKitLoggerDebug) Printf(format string, vars ...interface{}) {
	log.Debug("[ElasticSearch: Trace] => ", fmt.Sprintf(format, vars...))
}

type wrapKitLoggerError struct {
	log.Logger
}

func (logger wrapKitLoggerError) Printf(format string, vars ...interface{}) {
	log.Error("[ElasticSearch: Error] => ", fmt.Sprintf(format, vars...))
}
