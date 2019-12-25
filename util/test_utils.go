package util

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// TestURL for arc
var TestURL = "http://foo:bar@localhost:8000"

func StructToMap(response interface{}) interface{} {
	var mockMap map[string]interface{}
	marshalled, _ := json.Marshal(response)
	json.Unmarshal(marshalled, &mockMap)
	return mockMap
}

func MakeHttpRequest(method string, url string, requestBody interface{}) (interface{}, error) {
	var response interface{}
	finalURL := TestURL + url
	marshalledRequest, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorln("error while marshalling req body:", err)
		return nil, err
	}
	req, _ := http.NewRequest(method, finalURL, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorln("error while sending request:", err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return nil, err
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return response, err
	}
	return response, nil
}
