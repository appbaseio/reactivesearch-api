package users

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/util"
	log "github.com/sirupsen/logrus"
)

func subscribeToDowntimeAlert(email string) error {
	// Downtime alerts only work for cluster users
	if util.ClusterBilling == "true" && email != "" {
		clusterID := os.Getenv(util.ClusterIDEnvName)
		url := util.ACCAPI + "cluster/alert/" + clusterID
		requestBody, _ := json.Marshal(map[string]interface{}{
			"email": email,
		})
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("cache-control", "no-cache")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Errorln("error while subscribing to downtime alerts:", err)
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errors.New("error encountered while subscribing to down-time alerts")
		}
	}
	return nil
}

func unsubscribeToDowntimeAlert(email string) error {
	// Downtime alerts only work for cluster users
	if util.ClusterBilling == "true" {
		clusterID := os.Getenv(util.ClusterIDEnvName)
		url := util.ACCAPI + "cluster/alert/" + clusterID
		requestBody, _ := json.Marshal(map[string]interface{}{
			"email": email,
		})
		req, _ := http.NewRequest(http.MethodDelete, url, bytes.NewBuffer(requestBody))
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("cache-control", "no-cache")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Errorln("error while un-subscribing to downtime alerts:", err)
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errors.New("error encountered while un-subscribing to down-time alerts")
		}
	}
	return nil
}

func HasAction(actions []user.UserAction, action user.UserAction) bool {
	for _, c := range actions {
		if c == action {
			return true
		}
	}
	return false
}
