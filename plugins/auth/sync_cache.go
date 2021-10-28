package auth

import (
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/dgrijalva/jwt-go"
	"github.com/olivere/elastic/v7"
	"github.com/prometheus/common/log"
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

	var publicKey *publicKey
	publicKeys := util.GetHitsForIndex(response, s.index)

	for _, hit := range publicKeys {
		if hit.Id == publicKeyDocID {
			err := json.Unmarshal(hit.Source, publicKey)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return err
			}
			break
		}
	}

	if publicKey != nil {
		// update public key to cache if found
		publicKeyBuf, err := util.DecodeBase64Key(publicKey.PublicKey)
		if err != nil {
			log.Errorln(logTag, ":error parsing public key record,", err)
			return err
		}
		s.a.jwtRsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyBuf)
		if err != nil {
			log.Errorln(logTag, ":error parsing public key record,", err)
		}
		s.a.jwtRoleKey = publicKey.RoleKey
	}

	return nil
}
