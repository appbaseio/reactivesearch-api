package util

// isTracingEnabled will store the status of whether or
// not tracing should be enabled
var isTracingEnabled bool

// SetTracingEnabled will set the passed value into tracing
// enabled flag.
func SetTracingEnabled(value bool) {
	isTracingEnabled = value
}

// GetTracingEnabled will return the value of tracing
// enabled flag.
func GetTracingEnabled() bool {
	return isTracingEnabled
}

// serviceName will store the service name to use for
// tracing.
var serviceName string = "reactivesearch-api"

// SetServiceName will set the passed service name
func SetServiceName(value string) {
	serviceName = value
}

// GetServiceName will return the service name
func GetServiceName() string {
	return serviceName
}

// tracingEnv will store the tracing environment to use
var tracingEnv string = "prod"

// SetTracingEnv will set the tracing environment
func SetTracingEnv(value string) {
	tracingEnv = value
}

// GetTracingEnv will return the tracing environment
func GetTracingEnv() string {
	return tracingEnv
}
