package plugins

import (
	"fmt"
	"os"
)

// RSUtil will contain uril methods for
// working with RS internally.
type RSUtil struct {
}

// RSUtilInstance will return single instance for RSUtil
// This should be the only access for the RSUtil Instance
func RSUtilInstance() *RSUtil {
	rsUtilOnce.Do(func() { singletonRSUtil = &RSUtil{} })
	return singletonRSUtil
}

// URL will expose the RS Url for the current
// instance.
func (rsUtil *RSUtil) URL(withCredentials bool) string {
	ssl := "http"
	if rsUtil.IsHttps() {
		ssl += "s"
	}

	url := fmt.Sprint(ssl, "://")

	if withCredentials {
		url += fmt.Sprint(rsUtil.MasterCredentials(), "@")
	}

	return fmt.Sprintf("%s%s:%d", url, rsUtil.Address(), rsUtil.Port())
}

// Port will return the RS Port for the current instance
func (rsUtil *RSUtil) Port() int {
	healthCheckInstance := rsUtil.getHealthInstance()
	return *healthCheckInstance.port
}

// Address will return the RS Address for the current instance
func (rsUtil *RSUtil) Address() string {
	healthCheckInstance := rsUtil.getHealthInstance()
	return *healthCheckInstance.address
}

// IsHttps will return whether or not SSL is being used
func (rsUtil *RSUtil) IsHttps() bool {
	healthCheckInstance := rsUtil.getHealthInstance()
	return *healthCheckInstance.isHttps
}

// MasterCredentials will return the master credentials for
// the current instance in the username:password format.
func (rsUtil *RSUtil) MasterCredentials() string {
	username, password := os.Getenv("USERNAME"), os.Getenv("PASSWORD")
	if username == "" {
		username, password = "foo", "bar"
	}
	return username + ":" + password
}

// getHealthInstance will return the health instance
func (rsUtil *RSUtil) getHealthInstance() *RouterHealthCheck {
	return RouterHealthCheckInstance()
}
