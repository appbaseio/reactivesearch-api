package util

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	es7 "github.com/olivere/elastic/v7"
	"github.com/prometheus/common/log"
)

type slsInstanceDetails struct {
	NodeCount               int64                  `json:"node_count"`
	Description             string                 `json:"description"`
	SubscriptionID          string                 `json:"subscription_id"`
	SubscriptionCanceled    bool                   `json:"subscription_canceled"`
	Trial                   bool                   `json:"trial"`
	TrialValidity           int64                  `json:"trial_validity"`
	CreatedAt               int64                  `json:"created_at"`
	Tier                    string                 `json:"tier"`
	TierValidity            int64                  `json:"tier_validity"`
	TimeValidity            int64                  `json:"time_validity"`
	Metadata                map[string]interface{} `json:"metadata"`
	Domain                  string                 `json:"domain"`
	Counter                 int64                  `json:"counter"`
	ESVersion               string                 `json:"es_version"`
	ElasticSearchURL        string                 `json:"elasticsearch_url"`
	FeatureCustomEvents     bool                   `json:"feature_custom_events"`
	FeatureSuggestions      bool                   `json:"feature_suggestions"`
	FeatureRules            bool                   `json:"feature_rules"`
	FeatureTemplates        bool                   `json:"feature_templates"`
	FeatureFunctions        bool                   `json:"feature_functions"`
	FeatureSearchRelevancy  bool                   `json:"feature_search_relevancy"`
	FeatureSearchGrader     bool                   `json:"feature_search_grader"`
	FeatureEcommerce        bool                   `json:"feature_ecommerce"`
	FeatureCache            bool                   `json:"feature_cache"`
	FeaturePipelines        bool                   `json:"feature_pipelines"`
	FeatureUIBuilderPremium bool                   `json:"feature_uibuilder_premium"`
	NumberOfMachines        int64                  `json:"number_of_machines"`
	Backend                 *Backend               `json:"backend,omitempty"`
	CustomerID              string                 `json:"customer_id"`
	TenantID                string                 `json:"tenant_id"`
}

var slsInstancesByDomain = make(map[string]slsInstanceDetails)

// returns the SLS instance details for domain, domain must be in raw form
func GetSLSInstanceByDomain(domain string) *slsInstanceDetails {
	instanceDetails, ok := slsInstancesByDomain[domain]
	if ok {
		return &instanceDetails
	}
	return nil
}

// GetTenantForDomain will get the tenantID for the passed domain
func GetTenantForDomain(domain string) string {
	instanceDetails := GetSLSInstanceByDomain(domain)
	if instanceDetails == nil {
		return ""
	}
	return ""
}

func UpdateSLSInstances() {
	url := ACCAPI + "sls/instances"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	// Add auth header
	req.Header.Add("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))

	res, err := HTTPClient().Do(req)
	if err != nil {
		log.Errorln("error updating domain to sls instance map")
		return
	}
	if res != nil && res.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln("error reading res body to update domain to sls instance map:", err)
			return
		}
		var response []slsInstanceDetails
		err = json.Unmarshal(body, &response)
		if err != nil {
			log.Errorln("error while un-marshalling res body:", err)
			return
		}
		var tempDomainToSLSMap = make(map[string]slsInstanceDetails)
		for _, v := range response {
			tempDomainToSLSMap[v.Domain] = v
		}
		slsInstancesByDomain = tempDomainToSLSMap
	}
	defer res.Body.Close()
}

func SearchServiceWithAuth(s *es7.SearchService, req *http.Request) *es7.SearchService {
	if MultiTenant {
		if req != nil {
			domainInfo, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		} else {
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		}
	}
	return s
}

func IndexServiceWithAuth(s *es7.IndexService, req *http.Request) *es7.IndexService {
	if MultiTenant {
		if req != nil {
			domainInfo, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		} else {
			// Use master creds for internal sync cache requests
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		}
	}
	return s
}

func UpdateServiceWithAuth(s *es7.UpdateService, req *http.Request) *es7.UpdateService {
	if MultiTenant {
		if req != nil {
			domainInfo, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		} else {
			// Use master creds for internal sync cache requests
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		}
	}
	return s
}

func DeleteServiceWithAuth(s *es7.DeleteService, req *http.Request) *es7.DeleteService {
	if MultiTenant {
		if req != nil {
			domainInfo, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		} else {
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		}
	}
	return s
}
