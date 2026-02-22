package preferences

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
)

func savePreferences(ctx context.Context, indexName string, body map[string]interface{}) ([]byte, error) {
	// Get mappings
	mappings, err := mappingsOf(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf(`error fetching mappings of index "%s": %v`, indexName, err)
	}
	var err2 error
	switch util.GetVersion() {
	default:
		mappings["_meta"] = body
		_, err2 = util.GetClient7().PutMapping().
			Index(indexName).
			BodyJson(mappings).
			Do(ctx)
	}

	if err2 != nil {
		return nil, err2
	}

	return json.Marshal(map[string]interface{}{
		"message": "Preferences saved successfully.",
	})
}

func getPreferences(ctx context.Context, indexName string) ([]byte, error) {
	mappings, err := mappingsOf(ctx, indexName)
	if err != nil {
		return nil, fmt.Errorf(`error fetching mappings of index "%s": %v`, indexName, err)
	}
	switch util.GetVersion() {
	case 6:
		if mappings["_doc"].(map[string]interface{})["_meta"] == nil {
			return json.Marshal(map[string]interface{}{})
		}
		return json.Marshal(mappings["_doc"].(map[string]interface{})["_meta"])
	default:
		if mappings["_meta"] == nil {
			return json.Marshal(map[string]interface{}{})
		}
		return json.Marshal(mappings["_meta"])
	}
}

func mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	response, err := util.GetClient7().GetMapping().
		Index(indexName).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	if response != nil {
		result, found := response[indexName]
		if !found {
			return nil, fmt.Errorf(`mappings result for index "%s" not found`, indexName)
		}
		indexMappings, ok := result.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(`cannot cast indexMappings for index "%s" to map[string]interface{}`, indexName)
		}
		var mappings interface{}
		switch util.GetVersion() {
		case 6:
			mappings, found = indexMappings["mappings"].(map[string]interface{})["_doc"]
		default:
			mappings, found = indexMappings["mappings"]
		}
		if !found {
			return nil, fmt.Errorf(`mappings for index "%s" not found`, indexName)
		}
		m, ok := mappings.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(`cannot cast mappings for index "%s" to map[string]interface{}`, indexName)
		}
		return m, nil

	}
	return make(map[string]interface{}), nil
}
