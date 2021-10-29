package auth

import (
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/dgrijalva/jwt-go"
	"github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type CacheSyncScript struct {
	index string
	a     *Auth
}

func (s CacheSyncScript) Index() string {
	return s.index
}
func (s CacheSyncScript) PluginName() string {
	return singleton.Name()
}

func (s CacheSyncScript) SetCache(response *elastic.SearchResult) error {

	var pubicKeyResponse *publicKey
	publicKeys := util.GetHitsForIndex(response, s.index)

	for _, hit := range publicKeys {
		if hit.Id == publicKeyDocID {
			var publicKey publicKey
			err := json.Unmarshal(hit.Source, &publicKey)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return err
			}
			pubicKeyResponse = &publicKey
			break
		}
	}

	if pubicKeyResponse != nil {
		// update public key to cache if found
		publicKeyBuf, err := util.DecodeBase64Key(pubicKeyResponse.PublicKey)
		if err != nil {
			log.Errorln(logTag, ":error parsing public key record,", err)
			return err
		}
		s.a.jwtRsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyBuf)
		if err != nil {
			log.Errorln(logTag, ":error parsing public key record,", err)
		}
		s.a.jwtRoleKey = pubicKeyResponse.RoleKey
	}

	return nil
}
