package util

import (
	"fmt"

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
	log.Errorln("[ElasticSearch: Error] => ", fmt.Sprintf(format, vars...))
}
