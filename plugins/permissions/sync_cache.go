package permissions

import (
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type CacheSyncScript struct {
	index string
}

func (s CacheSyncScript) Index() string {
	return s.index
}
func (s CacheSyncScript) PluginName() string {
	return singleton.Name()
}

func (s CacheSyncScript) SetCache(response *elastic.SearchResult) error {
	permissionHits := util.GetHitsForIndex(response, s.index)

	// domain to username to permission map
	var permissionsMap = make(map[string]map[string]*permission.Permission)
	for _, permissionHit := range permissionHits {
		var userPermission permission.Permission
		err := json.Unmarshal(permissionHit.Source, &userPermission)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		domain := userPermission.Domain
		if _, ok := permissionsMap[domain]; ok {
			permissionsMap[domain][userPermission.Username] = &userPermission
		} else {
			permissionsMap[domain] = map[string](*permission.Permission){
				userPermission.Username: &userPermission,
			}
		}

	}
	// Update cached credentials
	for domain, _ := range auth.GetCachedCredentials() {
		for _, credential := range auth.GetCachedCredentialsByDomain(domain) {
			credentialAsPermission, ok := credential.(*permission.Permission)
			if ok {
				if domainMap, ok := permissionsMap[domain]; ok {
					if esPermission, ok := domainMap[credentialAsPermission.Username]; ok {
						// update permission to cache
						auth.SaveCredentialToCache(domain, credentialAsPermission.Username, esPermission)
					} else {
						// delete permission from cache
						auth.RemoveCredentialFromCache(domain, credentialAsPermission.Username)
					}
				}

			}
		}

	}
	return nil
}
