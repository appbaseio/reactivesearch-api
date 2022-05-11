package querytranslate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// ConvertFunc will convert the passed value into a
// string to be passed in Solr
type ConvertFunc func(interface{}) string

// RSToSolr contains a map of functions that will
// convert the passed RS Query value to it's equivalent
// Solr query.
var RSToSolr = map[string]ConvertFunc{
	"id": func(id interface{}) string { return id.(string) },
}

// validateRSToSolrKey will make sure that all the
// keys present in the reactivesearch request body
// are convertible to Solr equivalent.
//
// It is important to note that this method should only
// be invoked if the backend is set to Solr.
func validateRSToSolrKey(rsBody *[]Query) *Err {
	// If there is any non empty key in the rsBody that
	// is not present in RSToSolr map then we will have
	// to throw an error to let the user know that the
	// key is not supported.

	// Parse the query to a map in order to check if keys
	// are present.
	for _, query := range *rsBody {
		marshalledQuery, marshalErr := json.Marshal(query)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error occurred while marshalling query to map to validate keys for solr conversion: ", marshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return &Err{
				err:  errors.New(errMsg),
				code: http.StatusInternalServerError,
			}
		}

		var queryAsMap map[string]interface{}
		unmarshallErr := json.Unmarshal(marshalledQuery, &queryAsMap)

		if unmarshallErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling query to map to validate conversion of RS to Solr: ", unmarshallErr)
			log.Errorln(logTag, ": ", errMsg)
			return &Err{
				err:  errors.New(errMsg),
				code: http.StatusInternalServerError,
			}
		}

		for key := range queryAsMap {
			_, ok := RSToSolr[key]
			if !ok {
				// Key does not exist in RSToSolr but is passed
				// We cannot allow this, so just raise an error.
				errMsg := fmt.Sprintf("%s: key is not allowed since it is not supported by Solr", key)
				log.Warnln(logTag, ": ", errMsg)
				return &Err{
					err:  errors.New(errMsg),
					code: http.StatusBadRequest,
				}
			}
		}

	}

	return nil
}
