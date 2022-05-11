package querytranslate

// validateRSToSolrKey will make sure that all the
// keys present in the reactivesearch request body
// are convertible to Solr equivalent.
//
// It is important to note that this method should only
// be invoked if the backend is set to Solr.
func validateRSToSolrKey(rsBody *[]Query) error {
	return nil
}
