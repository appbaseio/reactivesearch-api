package proxy

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/util"

	appbase_errors "github.com/appbaseio/reactivesearch-api/errors"
)

type arcProxy struct {
	clusterID string
	arcID     string
	client    *http.Client
}

func initPlugin(arcID, clusterID string) (*arcProxy, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	ap := &arcProxy{clusterID, arcID, client}
	return ap, nil
}

func (ap *arcProxy) getArcID() (string, error) {
	var arcID string
	// If hosted billing is true replace the clusterID with the arcID
	if util.HostedBilling == "true" {
		arcID = os.Getenv(clusterUUID)
		if arcID == "" {
			return "", appbase_errors.NewEnvVarNotSetError(clusterUUID)
		}
	} else {
		appbaseID, err := util.GetAppbaseID()
		if err != nil {
			return "", err
		}
		arcID = appbaseID
	}
	return arcID, nil
}

func (ap *arcProxy) sendRequest(url, method string, reqBody []byte) ([]byte, int, error) {
	request, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		log.Errorln(proxyTag, ":", err)
		return nil, 0, err
	}
	response, err := ap.client.Do(request)
	if err != nil {
		log.Errorln(proxyTag, ":", err)
		return nil, 0, err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln(proxyTag, ":", err)
		return nil, 0, err
	}
	return body, response.StatusCode, nil
}
