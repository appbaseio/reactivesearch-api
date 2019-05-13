package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/util"
)

func (px *Proxy) postSubscription() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if px.arcID == "" {
			arcID, err := px.ap.getArcID()
			if err != nil {
				util.WriteBackError(w, "arcID not found", http.StatusBadRequest)
				return
			}
			px.arcID = arcID
		}
		if px.subID != "" {
			util.WriteBackError(w, "subscription for this arc instance already exists", http.StatusBadRequest)
			return
		}
		if px.subID == "" {
			subID, _ := px.ap.getSubID()
			if subID != "" {
				util.WriteBackError(w, "subscription for this arc instance already exists", http.StatusBadRequest)
				return
			}
			px.subID = subID
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		response, statusCode, err := px.ap.sendRequest(fmt.Sprint("https://accapi.appbase.io/arc/", px.arcID, "/subscription"), "POST", reqBody)
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, response, statusCode)
	}
}

func (px *Proxy) postMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if px.arcID == "" {
			arcID, err := px.ap.getArcID()
			if err != nil {
				util.WriteBackError(w, "arcID not found", http.StatusBadRequest)
				return
			}
			px.arcID = arcID
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		response, statusCode, err := px.ap.sendRequest(fmt.Sprint("https://accapi.appbase.io/arc/", px.arcID, "/metadata"), "POST", reqBody)
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, response, statusCode)
	}
}

func (px *Proxy) deleteSubscription() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if px.arcID == "" {
			arcID, err := px.ap.getArcID()
			if err != nil {
				util.WriteBackError(w, "arcID not found", http.StatusBadRequest)
				return
			}
			px.arcID = arcID
		}
		if px.subID == "" {
			subID, _ := px.ap.getSubID()
			if subID != "" {
				util.WriteBackError(w, "subscription ID not found", http.StatusBadRequest)
				return
			}
			px.subID = subID
		}
		defer req.Body.Close()
		payload := []byte{}
		payload, _ = ioutil.ReadAll(req.Body)
		response, statusCode, err := px.ap.sendRequest(fmt.Sprint("https://accapi.appbase.io/arc/", px.arcID, "/subscription"), "DELETE", payload)
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, response, statusCode)
	}
}

func (px *Proxy) getSubscription() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if px.arcID == "" {
			arcID, err := px.ap.getArcID()
			if err != nil {
				util.WriteBackError(w, "arcID not found", http.StatusBadRequest)
				return
			}
			px.arcID = arcID
		}
		if px.subID == "" {
			subID, _ := px.ap.getSubID()
			if subID != "" {
				util.WriteBackError(w, "subscription ID not found", http.StatusBadRequest)
				return
			}
			px.subID = subID
		}
		if px.email == "" {
			emailID, _ := px.ap.getEmail()
			if emailID != "" {
				util.WriteBackError(w, "arc user email ID not found", http.StatusBadRequest)
				return
			}
			px.email = emailID
		}
		response, statusCode, err := px.ap.sendRequest(fmt.Sprint("https://accapi.appbase.io/arc/instance/", px.email, "?arcid=", px.arcID), "GET", nil)
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if statusCode > 205 {
			util.WriteBackRaw(w, response, statusCode)
			return
		}
		arcDetails := getArcDetails{}
		err = json.Unmarshal(response, &arcDetails)
		if err != nil {
			log.Printf("%s: %v\n", proxyTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		marshalledResponse, _ := json.Marshal(arcDetails)
		util.WriteBackRaw(w, marshalledResponse, statusCode)
	}
}
