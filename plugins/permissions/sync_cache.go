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

	// username to permission map
	var permissionsMap = make(map[string]*permission.Permission)
	for _, permissionHit := range permissionHits {
		var permission permission.Permission
		err := json.Unmarshal(permissionHit.Source, &permission)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		permissionsMap[permission.Username] = &permission
	}
	// Update cached credentials
	for _, credential := range auth.GetCachedCredentials() {
		credentialAsPermission, ok := credential.(*permission.Permission)
		if ok {
			var ESPermission = permissionsMap[credentialAsPermission.Username]
			if ESPermission != nil {
				// update permission to cache
				auth.SaveCredentialToCache(credentialAsPermission.Username, permissionsMap[credentialAsPermission.Username])
			} else {
				// delete permission from cache
				auth.RemoveCredentialFromCache(credentialAsPermission.Username)
			}
		}
	}
	return nil
}
