package util

import (
	"net/http"
	"os"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	es7 "github.com/olivere/elastic/v7"
	"github.com/prometheus/common/log"
)

func SearchServiceWithAuth(s *es7.SearchService, req *http.Request) *es7.SearchService {
	if MultiTenant {
		if req != nil {
			domainName, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", *domainName)
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
			domainName, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", *domainName)
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
			domainName, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", *domainName)
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
			domainName, err := domain.FromContext(req.Context())
			if err != nil {
				log.Errorln("error while reading domain from context")
			} else {
				s.Header("X-REACTIVESEARCH-DOMAIN", *domainName)
			}
		} else {
			s.Header("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))
		}
	}
	return s
}
