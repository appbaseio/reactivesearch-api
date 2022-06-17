package util

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type ZincClient struct {
	URL        string
	Username   string
	Password   string
	AuthHeader string
}

var (
	clientInit *sync.Once
	zincClient *ZincClient
)

// GetZincData will return the zinc data from the
// environment.
//
// The return will be three strings:
// - URL
// - username
// - password
func GetZincData() (string, string, string) {
	zincURL := os.Getenv("ZINC_CLUSTER_URL")

	if zincURL == "" {
		log.Fatal("Error encountered: ", fmt.Errorf("ES_CLUSTER_URL must be set in the environment variables"))
	}

	username, password := "", ""

	if strings.Contains(zincURL, "@") {
		splitIndex := strings.LastIndex(zincURL, "@")
		protocolWithCredentials := strings.Split(zincURL[0:splitIndex], "://")
		credentials := protocolWithCredentials[1]
		protocol := protocolWithCredentials[0]
		host := zincURL[splitIndex+1:]

		credentialSeparator := strings.Index(credentials, ":")
		username = credentials[0:credentialSeparator]
		password = credentials[credentialSeparator+1:]

		zincURL = fmt.Sprintf("%s://%s", protocol, host)

	}
	return zincURL, username, password
}

// GetZincClient will return the zinc client and only
// init it once.
func GetZincClient() *ZincClient {
	// initialize the client if not present
	if zincClient == nil {
		initZincClient()
	}
	return zincClient
}

// initZincClient will initiate the zinc client
// by extracting the details from the env file.
func initZincClient() {
	zincURL, username, password := GetZincData()
	authHeader := ""

	if username != "" && password != "" {
		authHeader = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	}

	zincClient = &ZincClient{
		URL:        zincURL,
		Username:   username,
		Password:   password,
		AuthHeader: authHeader,
	}
}
