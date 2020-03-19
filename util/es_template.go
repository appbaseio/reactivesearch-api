package util

import (
	"context"
)

// SetDefaultIndexTemplate to set default template for indexes
func SetDefaultIndexTemplate() error {
	version := GetVersion()
	if version == 7 {
		response, err := GetClient7().IndexTemplateExists("default_temp").
			Do(context.Background())
		if err != nil || !response {
			defaultSetting := `{"template" : "*", "settings" : {"number_of_shards" : 1, "max_ngram_diff" : 8, "max_shingle_diff" : 8}}`
			_, err := GetClient7().IndexPutTemplate("default_temp").BodyString(defaultSetting).Do(context.Background())
			if err != nil {
				return err
			}
		}
	}
	return nil
}
