package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/util"
)

func (px *Proxy) postSubscription() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		test := req.URL.Query().Get("test")

		isTestEnabled := false
		if test == "true" {
			isTestEnabled = true
		}
		if util.Billing == "true" {
			if px.arcID == "" {
				arcID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
					return
				}
				px.arcID = arcID
			}

			reqBody, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Errorln(proxyTag, ":", err)
				util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
				return
			}
			defer req.Body.Close()
			URL := fmt.Sprint(util.ACCAPI, "arc/", px.arcID, "/subscription")
			if isTestEnabled {
				URL += "?test=true"
			}
			response, statusCode, err := px.ap.sendRequest(URL, "POST", reqBody)
			if err != nil {
				log.Errorln(proxyTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
			util.WriteBackRaw(w, response, statusCode)
			return
		}
		// handle for hosted billing
		if px.clusterID == "" {
			clusterID, err := px.ap.getArcID()
			if err != nil {
				util.WriteBackError(w, "CLUSTER_ID env was not found", http.StatusBadRequest)
				return
			}
			px.clusterID = clusterID
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		var URL = fmt.Sprint(util.ACCAPI, "v1/subscription/byoc/", px.clusterID)
		if isTestEnabled {
			URL += "?test=true"
		}
		response, statusCode, err := px.ap.sendRequest(URL, "POST", reqBody)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
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
				util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
				return
			}
			px.arcID = arcID
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		response, statusCode, err := px.ap.sendRequest(fmt.Sprint(util.ACCAPI, "arc/", px.arcID, "/metadata"), "POST", reqBody)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
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
				util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
				return
			}
			px.arcID = arcID
		}
		defer req.Body.Close()
		d := json.NewDecoder(req.Body)
		body := deleteArcSubscription{}
		err := d.Decode(&body)
		if err != nil {
			if err.Error() == "decoding err:  EOF" {
				log.Errorln(proxyTag, ": empty request body: ", err)
			}
		}
		payload, err := json.Marshal(body)
		if err != nil {
			log.Errorln(proxyTag, ": error while marshalling the body: ", err)
			util.WriteBackError(w, "unable to parse request", http.StatusBadRequest)
			return
		}
		response, statusCode, err := px.ap.sendRequest(fmt.Sprint(util.ACCAPI, "arc/", px.arcID, "/subscription"), "DELETE", payload)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
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
				util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
				return
			}
			px.arcID = arcID
		}
		URL := fmt.Sprint(util.ACCAPI, "arc/instances/?arcid=", px.arcID)
		response, statusCode, err := px.ap.sendRequest(URL, "GET", nil)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if statusCode > 205 {
			util.WriteBackRaw(w, response, statusCode)
			return
		}
		arcDetails := getArcDetails{}
		err = json.Unmarshal(response, &arcDetails)
		if len(arcDetails.ArcInstances) == 1 {
			arcDetails.ArcInstances[0].NodeCount = util.NodeCount
		}
		// Set clusterID to empty, don't expose it in public endpoints
		arcDetails.ArcInstances[0].ClusterID = ""
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		marshalledResponse, _ := json.Marshal(arcDetails)
		util.WriteBackRaw(w, marshalledResponse, statusCode)
	}
}

func (px *Proxy) getPlan() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if (util.Billing == "true" && !util.OfflineBilling) ||
			util.HostedBilling == "true" {
			if px.arcID == "" {
				arcID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
					return
				}
				px.arcID = arcID
			}
			url := fmt.Sprint(util.ACCAPI, "arc/instances/?arcid=", px.arcID)
			if util.HostedBilling == "true" {
				url = fmt.Sprint(util.ACCAPI, "byoc/", px.arcID)
			}
			var isCached bool
			response, statusCode, err := px.ap.sendRequest(url, "GET", nil)
			if err != nil || statusCode > 205 {
				log.Errorln(proxyTag, ":", err, statusCode)
				// Read plan details from file system
				contents, err := readPlanToFile()
				if err != nil {
					log.Errorln(logTag, "error while reading Appbase cache", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				response = contents
				isCached = true
			}
			arcDetails := getArcDetails{}
			err = json.Unmarshal(response, &arcDetails)
			if len(arcDetails.ArcInstances) == 0 {
				util.WriteBackError(w, "No arc instance found for the APPBASE_ID.", http.StatusBadRequest)
				return
			}
			if len(arcDetails.ArcInstances) == 1 {
				arcDetails.ArcInstances[0].NodeCount = util.NodeCount
			}
			if err != nil {
				log.Errorln(proxyTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
			var billingType = "arc"
			if util.HostedBilling == "true" {
				billingType = "hosted_arc"
			}

			var res map[string]interface{}
			marshalledResponse, _ := json.Marshal(arcDetails.ArcInstances[0])
			json.Unmarshal(marshalledResponse, &res)
			res["billing_type"] = billingType
			res["version"] = util.Version
			res["cached"] = isCached
			// Delete `cluster_id`, don't expose it in public endpoints
			delete(res, "cluster_id")
			finalResponse, _ := json.Marshal(res)
			util.WriteBackRaw(w, finalResponse, http.StatusOK)
		} else if util.ClusterBilling == "true" {
			if px.clusterID == "" {
				clusterID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "CLUSTER_ID not found", http.StatusBadRequest)
					return
				}
				px.clusterID = clusterID
			}
			var isCached bool
			response, statusCode, err := px.ap.sendRequest(fmt.Sprint(util.ACCAPI, "v1/plan/", px.clusterID), "GET", nil)
			if err != nil || statusCode > 205 {
				log.Errorln(proxyTag, ":", err, statusCode)
				// Read plan details from file system
				contents, err := readPlanToFile()
				if err != nil {
					log.Errorln(logTag, "error while reading Appbase cache", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				response = contents
				isCached = true
			}
			clusterDetails := ClusterDetails{}
			err = json.Unmarshal(response, &clusterDetails)

			var res map[string]interface{}
			marshalledResponse, _ := json.Marshal(clusterDetails.Plan)
			json.Unmarshal(marshalledResponse, &res)
			res["billing_type"] = "cluster"
			res["version"] = util.Version
			res["cached"] = isCached
			// Delete `cluster_id`, don't expose it in public endpoints
			delete(res, "cluster_id")
			finalResponse, _ := json.Marshal(res)
			util.WriteBackRaw(w, finalResponse, http.StatusOK)
		} else {
			// Return default tier if billing is not enabled
			var res = make(map[string]interface{})
			res["tier"] = util.ArcBasic
			if util.GetTier() != nil {
				if util.OfflineBilling {
					res["offline_billing"] = true
				}
				res["billing_type"] = "arc"
				res["tier"] = util.GetTier()
				res["billing"] = false
				res["version"] = util.Version
			}
			finalResponse, _ := json.Marshal(res)
			util.WriteBackRaw(w, finalResponse, http.StatusOK)
		}
	}
}

func (px *Proxy) getPlanFS() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		contents, err := readPlanToFile()
		if err != nil {
			log.Errorln(logTag, "error while reading Appbase cache", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, contents, http.StatusOK)
	}
}

func (px *Proxy) getCuratedInsights() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		URL := fmt.Sprint(util.ACCAPI, "arc/curated_insights/")

		if util.Billing == "true" || util.HostedBilling == "true" {
			if px.arcID == "" {
				arcID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
					return
				}
				px.arcID = arcID
			}
			URL += px.arcID
		} else if util.ClusterBilling == "true" {
			if px.clusterID == "" {
				clusterID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "CLUSTER_ID not found", http.StatusBadRequest)
					return
				}
				px.clusterID = clusterID
			}
			URL += px.clusterID
		}

		response, statusCode, err := px.ap.sendRequest(URL, "GET", nil)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, response, statusCode)
	}
}

func (px *Proxy) subscribeCuratedInsights() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		URL := fmt.Sprint(util.ACCAPI, "arc/")

		if util.Billing == "true" || util.HostedBilling == "true" {
			if px.arcID == "" {
				arcID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
					return
				}
				px.arcID = arcID
			}
			URL += px.arcID + "/curated_insights"
		} else if util.ClusterBilling == "true" {
			if px.clusterID == "" {
				clusterID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "CLUSTER_ID not found", http.StatusBadRequest)
					return
				}
				px.clusterID = clusterID
			}
			URL += px.clusterID + "/curated_insights"
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		test := req.URL.Query().Get("test")
		if test == "true" {
			URL += "?test=true"
		}
		response, statusCode, err := px.ap.sendRequest(URL, "POST", reqBody)

		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, response, statusCode)
	}
}

func (px *Proxy) unSubscribeCuratedInsights() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		URL := fmt.Sprint(util.ACCAPI, "arc/")
		if util.Billing == "true" || util.HostedBilling == "true" {
			if px.arcID == "" {
				arcID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
					return
				}
				px.arcID = arcID
			}
			URL += px.arcID + "/curated_insights"
		} else if util.ClusterBilling == "true" {
			if px.clusterID == "" {
				clusterID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "CLUSTER_ID not found", http.StatusBadRequest)
					return
				}
				px.clusterID = clusterID
			}
			URL += px.clusterID + "/curated_insights"
		}

		test := req.URL.Query().Get("test")
		if test == "true" {
			URL += "?test=true"
		}
		response, statusCode, err := px.ap.sendRequest(URL, "DELETE", nil)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, response, statusCode)
	}
}

func (px *Proxy) updatePaymentMethod() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		URL := fmt.Sprint(util.ACCAPI, "arc/payment/")
		if util.Billing == "true" || util.HostedBilling == "true" {
			if px.arcID == "" {
				arcID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "APPBASE_ID not found", http.StatusBadRequest)
					return
				}
				px.arcID = arcID
			}
			URL += px.arcID
		} else if util.ClusterBilling == "true" {
			if px.clusterID == "" {
				clusterID, err := px.ap.getArcID()
				if err != nil {
					util.WriteBackError(w, "CLUSTER_ID not found", http.StatusBadRequest)
					return
				}
				px.clusterID = clusterID
			}
			URL += px.clusterID
		}

		test := req.URL.Query().Get("test")
		if test == "true" {
			URL += "?test=true"
		}
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		response, statusCode, err := px.ap.sendRequest(URL, "PUT", reqBody)
		if err != nil {
			log.Errorln(proxyTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, response, statusCode)
	}
}
