package nodes

// pingES7 will ping ElasticSearch based on the passed machine ID
// with the current unix timestamp.
//
// This function will also determine whether the document should
// be created or updated based on the machineID being present
// or not being present in ES.
func (es *elasticsearch) pingES7(machineID string) error {
	return nil
}
