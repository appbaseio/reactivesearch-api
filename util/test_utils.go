package util

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"os/exec"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// TestURL for arc
var TestURL = "http://foo:bar@localhost:8000"

type BuildArc struct {
	cmd                 *exec.Cmd
	Tier                Plan
	FeatureCustomEvents bool
	FeatureSuggestions  bool
}

func StartArc(b *BuildArc) BuildArc {
	ldFlags := "export TEST_TIER=" + b.Tier.String() + " TEST_FEATURE_CUSTOM_EVENTS=" + strconv.FormatBool(b.FeatureCustomEvents) + " TEST_FEATURE_SUGGESTIONS=" + strconv.FormatBool(b.FeatureSuggestions) + ";"
	makeCmd := exec.Command("/bin/sh", "-c", "cd ..; cd ..;"+ldFlags+"make clean; make;")
	err := makeCmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	buildCmd := exec.Command("/bin/sh", "-c", "cd ..; cd ..; ./build/arc --env=config/manual.env;")
	buildCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err2 := buildCmd.Start()
	if err2 != nil {
		log.Fatal(err2)
	}
	b.cmd = buildCmd
	return *b
}

func (b *BuildArc) Start() {
	b.cmd.Start()
	time.Sleep(time.Duration(20) * time.Second)
}

func (b *BuildArc) Close() {
	err := syscall.Kill(-b.cmd.Process.Pid, syscall.SIGKILL)
	if err != nil {
		log.Fatal("Unable to kill process", err)
	}
}

func StructToMap(response interface{}) interface{} {
	var mockMap map[string]interface{}
	marshalled, _ := json.Marshal(response)
	json.Unmarshal(marshalled, &mockMap)
	return mockMap
}

func MakeHttpRequest(method string, url string, requestBody interface{}) (interface{}, error, *http.Response) {
	var response interface{}
	finalURL := TestURL + url
	marshalledRequest, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorln("error while marshalling req body:", err)
		return nil, err, nil
	}
	req, _ := http.NewRequest(method, finalURL, bytes.NewBuffer(marshalledRequest))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorln("error while sending request:", err)
		return nil, err, nil
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorln("error reading res body:", err)
		return nil, err, nil
	}
	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Errorln("error while unmarshalling res body:", err)
		return response, err, nil
	}
	return response, nil, res
}
