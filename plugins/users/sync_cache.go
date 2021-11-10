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

	// username to user map
	var usersMap = make(map[string]*user.User)
	for _, userHit := range userHits {
		var user user.User
		err := json.Unmarshal(userHit.Source, &user)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		usersMap[user.Username] = &user
	}
	// Update cached credentials
	for _, credential := range auth.GetCachedCredentials() {
		credentialAsUser, ok := credential.(*user.User)
		if ok {
			var ESuser = usersMap[credentialAsUser.Username]
			if ESuser != nil {
				// detect change in password and delete the cached password
				if credentialAsUser.Password != ESuser.Password {
					// clear cached valid password
					auth.ClearPassword(credentialAsUser.Username)
				}
				// update user to cache
				auth.SaveCredentialToCache(credentialAsUser.Username, usersMap[credentialAsUser.Username])
			} else {
				// delete user from cache
				auth.RemoveCredentialFromCache(credentialAsUser.Username)
				// clear cached valid password
				auth.ClearPassword(credentialAsUser.Username)
			}
		}
	}
	return nil
}
