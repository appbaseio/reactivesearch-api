package users

import (
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/model/user"
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
	userHits := util.GetHitsForIndex(response, s.index)

	// domain to username to permission map
	var usersMap = make(map[string]map[string]*user.User)
	for _, userHit := range userHits {
		var userPermission user.User
		err := json.Unmarshal(userHit.Source, &userPermission)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		if _, ok := usersMap[userPermission.Domain]; ok {
			usersMap[userPermission.Domain][userPermission.Username] = &userPermission
		} else {
			usersMap[userPermission.Domain] = map[string](*user.User){
				userPermission.Username: &userPermission,
			}
		}
	}
	// Update cached credentials
	for domain, _ := range auth.GetCachedCredentials() {
		for _, credential := range auth.GetCachedCredentialsByDomain(domain) {
			credentialAsUser, ok := credential.(*user.User)
			if ok {
				if domainMap, ok := usersMap[domain]; ok {
					if esUser, ok := domainMap[credentialAsUser.Username]; ok {
						// update permission to cache
						auth.SaveCredentialToCache(domain, credentialAsUser.Username, esUser)
					} else {
						// delete permission from cache
						auth.RemoveCredentialFromCache(domain, credentialAsUser.Username)
					}
				}

			}
		}

	}

	return nil
}
