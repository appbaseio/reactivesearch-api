package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// Use it to set the default value of tenant/domain in cache for single tenant
const DefaultTenant = "reactivesearch.io"

type slsInstanceDetails struct {
	NodeCount               int64                  `json:"node_count"`
	Description             string                 `json:"description"`
	SubscriptionID          string                 `json:"subscription_id"`
	SubscriptionCanceled    bool                   `json:"subscription_canceled"`
	Trial                   bool                   `json:"trial"`
	TrialValidity           int64                  `json:"trial_validity"`
	CreatedAt               int64                  `json:"created_at"`
	Tier                    *Plan                  `json:"tier"`
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

// Store the domains where the payment is required. Essentially, store
// all the domains where the `pricing_plan` field is not valid.
var slsDomainsPaymentNeeded = make(map[string]int)

// returns the SLS instance details for domain, domain must be in raw form
func GetSLSInstanceByDomain(domain string) *slsInstanceDetails {
	instanceDetails, ok := slsInstancesByDomain[domain]
	if ok {
		return &instanceDetails
	}
	return nil
}

// IsPaymentNeeded will check if the passed domain requires payment
func IsPaymentNeeded(domain string) bool {
	_, exists := slsDomainsPaymentNeeded[domain]
	return exists
}

// GetSLSInstances will return the domain to SLS instance details map
func GetSLSInstances() map[string]slsInstanceDetails {
	return slsInstancesByDomain
}

// GetTenantForDomain will get the tenantID for the passed domain
func GetTenantForDomain(domain string) string {
	instanceDetails := GetSLSInstanceByDomain(domain)
	if instanceDetails == nil {
		return ""
	}
	return instanceDetails.TenantID
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

		// Once the response is present in the list, remove all the instances
		// that do not have a valid plan
		indicesToRemove := make([]int, 0)
		for instancePosition, instance := range response {
			if *instance.Tier == InvalidValueEncountered {
				log.Warnln("removing instance with domain `", instance.Domain, "` from sls instances since it has an invalid plan: ")
				// Remove the element from the index
				indicesToRemove = append(indicesToRemove, instancePosition)
			}
		}

		// Remove all the indices that are to be removed
		//
		// Empty the older map on sync
		slsDomainsPaymentNeeded = make(map[string]int)
		for _, instancePosition := range indicesToRemove {
			// Before removing them from valid plans, we also want to
			// keep them in a separate list so that we can throw a proper
			// error to the user when they try to make a request
			instanceDetails := response[instancePosition]
			slsDomainsPaymentNeeded[instanceDetails.Domain] = 1

			response[instancePosition] = response[len(response)-1]
			response = response[:len(response)-1]
		}

		var tempDomainToSLSMap = make(map[string]slsInstanceDetails)
		for _, v := range response {
			tempDomainToSLSMap[v.Domain] = v
		}
		slsInstancesByDomain = tempDomainToSLSMap
	}
	defer res.Body.Close()

	// Update the request map in case new tenants are added or older tenants
	// changed their plans
	InitRequestMap()
}

func SearchServiceWithAuth(s *es7.SearchService, ctx context.Context) *es7.SearchService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func IndexServiceWithAuth(s *es7.IndexService, ctx context.Context) *es7.IndexService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func UpdateServiceWithAuth(s *es7.UpdateService, ctx context.Context) *es7.UpdateService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func DeleteServiceWithAuth(s *es7.DeleteService, ctx context.Context) *es7.DeleteService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func GetServiceWithAuth(s *es7.GetService, ctx context.Context) *es7.GetService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func ValidateServiceWithAuth(s *es7.ValidateService, ctx context.Context) *es7.ValidateService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func BulkServiceWithAuth(s *es7.BulkService, ctx context.Context) *es7.BulkService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func CountServiceWithAuth(s *es7.CountService, ctx context.Context) *es7.CountService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func AliasesServiceWithAuth(s *es7.AliasesService, ctx context.Context) *es7.AliasesService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func DeleteByQueryServiceWithAuth(s *es7.DeleteByQueryService, ctx context.Context) *es7.DeleteByQueryService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

func IndicesGetMappingServiceWithAuth(s *es7.IndicesGetMappingService, ctx context.Context) *es7.IndicesGetMappingService {
	if MultiTenant {
		if ctx == nil || ctx == context.Background() {
			// user master creds if context is nil
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		} else {
			domainInfo, err := domain.FromContext(ctx)
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", domainInfo.Encrypted)
			}
		}
	}
	return s
}

// GetESClientForTenant will get the esClient for the tenant so that
// it can be used to make requests
func GetESClientForTenant(ctx context.Context) (*es7.Client, error) {
	if !MultiTenant {
		return GetClient7(), nil
	}
	if ctx == nil || ctx == context.Background() {
		return GetSystemClient()
	}
	// Check the backend and accordingly determine the client.
	domain, domainFetchErr := domain.FromContext(ctx)
	if domainFetchErr != nil {
		errMsg := fmt.Sprintf("error while fetching domain info from context: %s", domainFetchErr.Error())
		return nil, fmt.Errorf(errMsg)
	}

	backend := GetBackendByDomain(domain.Raw)
	if *backend == System {
		return GetSystemClient()
	}

	// If backend is not `system`, this route can be called for an ES
	// backend only.
	//
	// We will have to fetch the ES_URL value from global vars and create
	// a simple client using that.

	// Fetch the tenantId using the domain
	tenantId := GetTenantForDomain(domain.Raw)
	esAccess := GetESAccessForTenant(tenantId)

	if esAccess.URL == "" {
		errMsg := "ES_URL not defined in global vars, cannot continue without that!"
		return nil, fmt.Errorf(errMsg)
	}

	// NOTE: We are assuming that basic auth will be provided in the URL itself.
	//
	// We will deprecate support for ES_HEADER since pipelines can work without
	// the header as well by accepting basic auth in the URL itself.
	esURLParsed, parseErr := ParseESURL(esAccess.URL, esAccess.Header)
	if parseErr != nil {
		errMsg := fmt.Sprint("Error while parsing ES_URL and ES_HEADER: ", parseErr.Error())
		return nil, fmt.Errorf(errMsg)
	}

	loggerT := log.New()
	wrappedLoggerDebug := &WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &WrapKitLoggerError{*loggerT}

	esClient, clientErr := es7.NewSimpleClient(
		es7.SetURL(esURLParsed),
		es7.SetRetrier(NewRetrier()),
		es7.SetHttpClient(HTTPClient()),
		es7.SetErrorLog(wrappedLoggerError),
		es7.SetInfoLog(wrappedLoggerDebug),
		es7.SetTraceLog(wrappedLoggerDebug),
	)

	if clientErr != nil {
		errMsg := fmt.Sprint("error while initiating client to make request: ", clientErr.Error())
		return nil, fmt.Errorf(errMsg)
	}

	return esClient, nil
}
