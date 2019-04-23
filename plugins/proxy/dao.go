package proxy

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"bytes"
	"crypto/tls"

	appbase_errors "github.com/appbaseio-confidential/arc/errors"
)

type arcProxy struct {
	arcID  string
	subID  string
	email string
	client *http.Client
}

func newClient(arcID, subID, email string) (*arcProxy, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify : true},
	}
	client := &http.Client{Transport: tr}
	ap := &arcProxy{arcID, subID,email, client}
	return ap, nil
}

func (ap *arcProxy) getArcID() (string, error) {
	arcID := os.Getenv(arcUUID)
	if arcID == "" {
		return "", appbase_errors.NewEnvVarNotSetError(arcUUID)
	}
	return arcID, nil
}

func (ap *arcProxy) getEmail() (string, error) {
	arcID := os.Getenv(arcUUID)
	if arcID == "" {
		return "", appbase_errors.NewEnvVarNotSetError(arcUUID)
	}
	return arcID, nil
}

func (ap *arcProxy) getSubID() (string, error) {
	subscriptionID := os.Getenv(subID)
	if subscriptionID == "" {
		return "", appbase_errors.NewEnvVarNotSetError(subID)
	}
	return subscriptionID, nil
}


func (ap *arcProxy) sendRequest(url, method string, reqBody []byte) ([]byte, int, error) {
	request, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			return nil, 0 , err
		}
	response, err := ap.client.Do(request)
	if err != nil {
		log.Printf("%s: %v\n", proxyTag, err)
		return nil, 0 , err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("%s: %v\n", proxyTag, err)
		return nil, 0 , err
	}
	return body, response.StatusCode, nil
}