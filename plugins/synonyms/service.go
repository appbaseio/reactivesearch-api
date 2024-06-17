package synonyms

import "context"

type synonymsService interface {
	putSynonyms(ctx context.Context, record []SynonymsStruct, index string) (error, []SynonymsStruct)
	deleteSynonyms(ctx context.Context, docID string) error
	getSynonyms(ctx context.Context, indexName string) ([]SynonymsStruct, error)
	deleteAllSynonyms(ctx context.Context, index string) error
}
