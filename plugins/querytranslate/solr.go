package querytranslate

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
func validateRSToSolrKey(rsBody *[]Query) error {
	// If there is any non empty key in the rsBody that
	// is not present in RSToSolr map then we will have
	// to throw an error to let the user know that the
	// key is not supported.
	return nil
}
